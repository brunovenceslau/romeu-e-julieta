// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package salvage holds what both binaries know of the bundles that
// carry a sandbox's unpushed work to the host (08 8.4 and 8.5): the
// refs a snapshot bundles (Pairs, REQ-0688) and the format of
// snapshot/heads.json, its writer's encoder and romeu's strict reader
// (EncodeHeads, DecodeHeads; 03 3.13, REQ-0261). It writes nothing to
// a memory dir: julieta's writer, which takes the store lock, is
// internal/hooks/actions (snapshot.go), since this package is linked by
// romeu too (10 10.7) and so imports no julieta-only package.
package salvage

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// The files of a snapshot, in the snapshot/ dir of a memory dir (08
// 8.4, REQ-0689).
const (
	// BundleFile is the bundle, renamed into place first.
	BundleFile = "heads.bundle"
	// HeadsFile is snapshot/v1, written after the bundle.
	HeadsFile = "heads.json"
)

// HeadsVersion is the version of snapshot/heads.json this build writes
// and reads.
const HeadsVersion = 1

// HeadsField is the value of the schema field of snapshot/heads.json,
// "snapshot/v<HeadsVersion>" (03, opening, REQ-0161).
var HeadsField = "snapshot/v" + strconv.Itoa(HeadsVersion)

// headsSchema reads a schema field of the form "snapshot/v<n>".
var headsSchema = regexp.MustCompile(`^snapshot/v([1-9][0-9]{0,8})$`)

// ErrInvalid is the error of a snapshot/heads.json this build does not
// read: it does not decode strictly, misses a member, names another
// format or an older version, holds a value of the wrong form or passes
// the file cap. A newer version is format-newer (RJ-311) instead.
var ErrInvalid = errors.New("salvage: not a snapshot/heads.json")

// Heads is a snapshot/heads.json document (08 8.4, REQ-0689): every ref
// the snapshot read with the object it points at, the time it was
// taken, and whether the repo's own core.hooksPath bypasses the
// dispatcher, so no commit takes a snapshot (REQ-0692). romeu trusts
// the bundle's own refs and reads this file for status only: a ref an
// origin SHA reaches is listed here and left out of the bundle, which
// holds only the objects beyond repos[].base.
type Heads struct {
	// CreatedAt is when the snapshot was taken, UTC, to the second.
	CreatedAt time.Time `json:"createdAt"`
	// Disabled says that the repo's local config sets core.hooksPath.
	Disabled bool `json:"disabled"`
	// Refs maps each full ref name to its full object id.
	Refs map[string]string `json:"refs"`
	// Schema is HeadsField.
	Schema string `json:"schema"`
}

// BundleNamespaces are the namespaces of the refs a snapshot bundles
// (REQ-0688): the user's branches, tags and notes, the current
// generation's salvage refs, and the names the snapshot gives each
// reflog-only commit and each detached worktree HEAD.
var BundleNamespaces = []string{"refs/heads/", "refs/tags/", "refs/notes/", "refs/salvage/", ReflogPrefix}

// ReflogPrefix is the namespace of the names of the reflog-only commits
// and the detached worktree HEADs of a snapshot.
const ReflogPrefix = "refs/salvage-reflog/"

// objectID is a full object id of either hash, lowercase.
var objectID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// EncodeHeads returns the bytes of h, with its schema set to HeadsField
// and its time stamped (memstore.Stamp): JSON with its members sorted
// and a final line feed. A document DecodeHeads would refuse is
// refused here too, wrapping ErrInvalid, so the writer never writes
// what romeu cannot read.
func EncodeHeads(h Heads) ([]byte, error) {
	h.Schema = HeadsField
	h.CreatedAt = memstore.Stamp(h.CreatedAt)
	if h.Refs == nil {
		h.Refs = map[string]string{}
	}
	if err := checkHeads(h); err != nil {
		return nil, err
	}
	data, err := json.Marshal(h, json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	if len(data) > memstore.MaxFileSize {
		return nil, fmt.Errorf("%w: %d bytes pass the file cap of %d", ErrInvalid, len(data), memstore.MaxFileSize)
	}
	return data, nil
}

// DecodeHeads reads snapshot/heads.json strictly (03, opening,
// REQ-0160, REQ-0168): at most the file cap (memstore.MaxFileSize), a
// duplicate member name, text that is not UTF-8 and trailing data are
// each an error, and every member must be present and not null. A
// member this build does not know is skipped, as a newer julieta of the
// same version may add one (REQ-0163). The schema is read first, so a
// newer version is format-newer (RJ-311) even where it holds members
// this build does not know. Then createdAt is a UTC time, each ref is under
// BundleNamespaces with a name git and the terminal accept, and each
// object id is full and lowercase. Every other refusal wraps
// ErrInvalid.
func DecodeHeads(data []byte) (Heads, error) {
	if len(data) > memstore.MaxFileSize {
		return Heads{}, fmt.Errorf("%w: %d bytes pass the file cap of %d", ErrInvalid, len(data), memstore.MaxFileSize)
	}
	var head struct {
		Schema *string `json:"schema"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return Heads{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if head.Schema == nil {
		return Heads{}, fmt.Errorf("%w: no schema member", ErrInvalid)
	}
	sub := headsSchema.FindStringSubmatch(*head.Schema)
	if sub == nil {
		return Heads{}, fmt.Errorf("%w: the schema %q is not snapshot/v<n>", ErrInvalid, clip(*head.Schema))
	}
	switch n, _ := strconv.Atoi(sub[1]); {
	case n > HeadsVersion:
		return Heads{}, manifest.FormatNewer.New(fmt.Sprintf("snapshot/heads.json is %s, newer than %s, the version this build reads", *head.Schema, HeadsField))
	case n < HeadsVersion:
		return Heads{}, fmt.Errorf("%w: the schema %s is older than %s", ErrInvalid, *head.Schema, HeadsField)
	}
	var doc struct {
		CreatedAt *time.Time         `json:"createdAt"`
		Disabled  *bool              `json:"disabled"`
		Refs      *map[string]string `json:"refs"`
		Schema    string             `json:"schema"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return Heads{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if doc.CreatedAt == nil || doc.Disabled == nil || doc.Refs == nil || *doc.Refs == nil {
		return Heads{}, fmt.Errorf("%w: createdAt, disabled and refs are each required", ErrInvalid)
	}
	h := Heads{CreatedAt: *doc.CreatedAt, Disabled: *doc.Disabled, Refs: *doc.Refs, Schema: doc.Schema}
	if err := checkHeads(h); err != nil {
		return Heads{}, err
	}
	return h, nil
}

// checkHeads returns ErrInvalid for a value of h that is not of its
// form.
func checkHeads(h Heads) error {
	if h.CreatedAt.IsZero() || h.CreatedAt.Location() != time.UTC {
		return fmt.Errorf("%w: createdAt %s is not a UTC time", ErrInvalid, h.CreatedAt.Format(time.RFC3339))
	}
	for _, name := range slices.Sorted(maps.Keys(h.Refs)) {
		if err := CheckRef(name); err != nil {
			return err
		}
		if oid := h.Refs[name]; !objectID.MatchString(oid) {
			return fmt.Errorf("%w: the ref %q points at %q, not a full object id", ErrInvalid, clip(name), clip(oid))
		}
	}
	return nil
}

// CheckRef returns ErrInvalid for a name that is not the name of a ref
// a snapshot bundles: under one of BundleNamespaces, with a name after
// it, and with nothing git check-ref-format refuses (.., @{, a
// component that starts with a dot or ends with .lock, an empty
// component, a final dot, a space or one of ~^:?[\*) and no character
// that is a control or a format character, which a terminal could be
// made to misread (I29).
func CheckRef(name string) error {
	refuse := func(why string) error {
		return fmt.Errorf("%w: the ref name %q %s", ErrInvalid, clip(name), why)
	}
	if !slices.ContainsFunc(BundleNamespaces, func(ns string) bool { return strings.HasPrefix(name, ns) && len(name) > len(ns) }) {
		return refuse("is not below " + strings.Join(BundleNamespaces, ", "))
	}
	switch {
	case !utf8.ValidString(name):
		return refuse("is not valid UTF-8")
	case strings.Contains(name, "..") || strings.Contains(name, "@{"):
		return refuse("holds .. or @{")
	case strings.HasSuffix(name, "."):
		return refuse("ends with a dot")
	case strings.ContainsFunc(name, func(r rune) bool {
		return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == 0x2028 || r == 0x2029 || strings.ContainsRune(" ~^:?[\\*", r)
	}):
		return refuse("holds a character git or the terminal refuses")
	}
	for c := range strings.SplitSeq(name, "/") {
		if c == "" || strings.HasPrefix(c, ".") || strings.HasSuffix(c, ".lock") {
			return refuse("has an empty component, or one that starts with a dot or ends with .lock")
		}
	}
	return nil
}

// clipLen is the most bytes of a value an error quotes.
const clipLen = 64

// clip returns s cut to clipLen bytes, so an error quotes no long value.
func clip(s string) string {
	if len(s) <= clipLen {
		return s
	}
	cut := clipLen
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "..."
}
