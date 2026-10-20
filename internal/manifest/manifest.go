// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package manifest is the romeu/julieta contract of 03 3.6, the format
// manifest.v1: the one Go type both binaries import for it (REQ-0217),
// its strict decoder with the protocol window N and N-1 (REQ-0219), its
// canonical encoder and the size rule of the env value.
//
// romeu passes a manifest on every sbx env exec as
// JULIETA_MANIFEST=<base64url(canonical JSON)> (REQ-0215), written by
// EncodeEnv and read by DecodeEnv; julieta caches the canonical JSON,
// which Encode writes and Decode reads. The manifest is written by a
// program, never by hand, so Decode reads only the bytes Encode writes:
// a document in any other spelling is refused, and no difference
// between the parsers of the writer and the reader can arise.
package manifest

import (
	"bytes"
	"encoding/base64"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// The size rule of 03 3.6 (REQ-0216): sync and julieta spec validate
// refuse a manifest whose env value is over MaxEncodedSize bytes, and
// the decoders read no more.
const (
	// MaxEncodedSize is the cap of the env value, 32 KiB.
	MaxEncodedSize = 32 << 10
	// MaxJSONSize is the most bytes of canonical JSON whose env value
	// fits the cap.
	MaxJSONSize = MaxEncodedSize / 4 * 3
)

// Manifest is a manifest.v1 document (03 3.6). The fields of every type
// of the format are declared in the order of their JSON names, sorted,
// so the type reads in the order Encode writes; Encode sorts them
// itself.
type Manifest struct {
	// Agent names the agent profile of the sandbox.
	Agent string `json:"agent"`
	// Egress is the set of domains romeu applied to this sandbox, auto
	// plus approved gated (REQ-0224).
	Egress []string `json:"egress"`
	// Generation is the sandbox generation's lowercase ULID.
	Generation string `json:"generation"`
	// Project is the project's name.
	Project string `json:"project"`
	// Protocol is the romeu/julieta contract number (REQ-0218).
	Protocol int `json:"protocol"`
	// Repos are the project's repositories, the primary among them.
	Repos []Repo `json:"repos"`
	// Run is the validated run layout of 03 3.2.
	Run Run `json:"run"`
	// RunDigest is the digest of kind run of Run (03 3.12), which Decode
	// checks.
	RunDigest string `json:"runDigest"`
	// Salvage holds the salvage settings.
	Salvage Salvage `json:"salvage"`
	// SalvageRun is present only on salvage calls (REQ-0223).
	SalvageRun *SalvageRun `json:"salvageRun,omitzero"`
	// Schema is "manifest/v1" (REQ-0161).
	Schema string `json:"schema"`
	// SpecCommit is the config repo commit of the live spec, fresh at
	// each exec (REQ-0220).
	SpecCommit string `json:"specCommit"`
}

// Repo is one repository of the project. Decode checks the form of
// Base and the one primary; it does not apply the rules of 03 3.1 to
// Dir, URL and Ref, nor check Path and Memory, which 03 3.6 leaves to
// the validators of the project spec. A consumer validates each before
// it becomes a path, a ref, a URL or an argv element (REQ-0271): URL
// with gitsafe.CheckURL, the others with the validators of 03 3.1.
type Repo struct {
	// Base are the origin SHAs the host clone already has (REQ-0222).
	Base []string `json:"base"`
	// Dir is the repository's directory name in the project.
	Dir string `json:"dir"`
	// Memory is the path of the repository's memory directory.
	Memory string `json:"memory"`
	// Path is where the repository is inside the sandbox (REQ-0221).
	Path string `json:"path"`
	// Primary says whether this is the primary repository.
	Primary bool `json:"primary"`
	// Ref is the branch the repository follows.
	Ref string `json:"ref"`
	// URL is the repository's origin.
	URL string `json:"url"`
}

// Salvage holds the salvage settings of the project.
type Salvage struct {
	// CapBytes is the size cap of a salvage.
	CapBytes int64 `json:"capBytes"`
	// ExcludeIgnored are the ignored paths a salvage leaves out.
	ExcludeIgnored []string `json:"excludeIgnored"`
}

// Run is the validated run layout of 03 3.2, as an object. Its Go type
// lands with internal/spec, and this field then takes it; until then a
// number in it is a float64, so a document that spells one in a form
// Encode does not write, or past what a float64 holds exactly, is not
// canonical and Decode refuses it.
type Run map[string]any

// CanonKind declares the kind of Run, run (03 3.12).
func (Run) CanonKind() canon.RunKind { return canon.Run }

// SalvageRun is the salvage romeu expects back (REQ-0223).
type SalvageRun struct {
	// ID is the salvage id.
	ID string `json:"id"`
}

// Field is the value of the schema field this build writes and reads,
// "manifest/v<Version>".
var Field = "manifest/v" + strconv.Itoa(Version)

// schemaField reads a schema field of the form "manifest/v<n>", n
// without a leading zero.
var schemaField = regexp.MustCompile(`^manifest/v([1-9][0-9]{0,8})$`)

// ErrInvalid is the error of a document that is not a manifest this
// build reads: it does not decode strictly, is not in the canonical
// form Encode writes, holds a value of the wrong form, names another
// format or an older version, or passes the size cap.
var ErrInvalid = errors.New("manifest: not a manifest")

// ErrTooLarge is the error of a manifest whose env value is over
// MaxEncodedSize bytes (REQ-0216).
var ErrTooLarge = errors.New("manifest: the env value passes the cap")

// The forms of the values Decode checks.
var (
	// commitForm is a commit id: 40 lowercase hexadecimal digits.
	commitForm = regexp.MustCompile(`^[0-9a-f]{40}$`)
	// digestForm is the text of a canon.Sum: 64 of them.
	digestForm = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Decode reads the canonical JSON of a manifest, strictly (03,
// opening, REQ-0160): an unknown member, a duplicate member name, a
// member name of another case, text that is not UTF-8 or holds U+FFFD,
// and trailing data are each an error. The schema field is read first,
// so a newer version is format-newer (RJ-311) even where it holds
// members this build does not know. Then the document must be the
// bytes Encode writes for it, exactly, so a missing member, a null,
// another key order, whitespace, another escape or number spelling, and
// a trailing newline are each refused. Then a protocol outside N and
// N-1 is manifest-protocol (RJ-342), and the values must have their
// forms (check). Every other refusal wraps ErrInvalid.
func Decode(data []byte) (Manifest, error) {
	if len(data) > MaxJSONSize {
		return Manifest{}, fmt.Errorf("%w: %d bytes of JSON pass the cap of %d", ErrInvalid, len(data), MaxJSONSize)
	}
	if err := validText(data); err != nil {
		return Manifest{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	var head struct {
		Schema *string `json:"schema"`
	}
	if err := jsonv2.Unmarshal(data, &head); err != nil {
		return Manifest{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if head.Schema == nil {
		return Manifest{}, fmt.Errorf("%w: no schema member", ErrInvalid)
	}
	sub := schemaField.FindStringSubmatch(*head.Schema)
	if sub == nil {
		return Manifest{}, fmt.Errorf("%w: the schema %q is not manifest/v<n>", ErrInvalid, clip(*head.Schema))
	}
	switch n, _ := strconv.Atoi(sub[1]); {
	case n > Version:
		return Manifest{}, FormatNewer.New(fmt.Sprintf("the manifest is %s, newer than %s, the version this build reads", *head.Schema, Field))
	case n < Version:
		return Manifest{}, fmt.Errorf("%w: the schema %s is older than %s", ErrInvalid, *head.Schema, Field)
	}
	var m Manifest
	if err := jsonv2.Unmarshal(data, &m, jsonv2.RejectUnknownMembers(true)); err != nil {
		return Manifest{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	enc, err := Encode(m)
	if err != nil {
		return Manifest{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if !bytes.Equal(enc, data) {
		return Manifest{}, fmt.Errorf("%w: the document is not the canonical JSON Encode writes for it (a member missing or null, another order, spacing or spelling)", ErrInvalid)
	}
	if !acceptsProtocol(m.Protocol, Protocol) {
		e := ManifestProtocol.New(fmt.Sprintf("the manifest is of protocol %d, and this julieta accepts protocol %d and the one before it", m.Protocol, Protocol),
			cli.Detail{Key: "protocol", Value: strconv.Itoa(m.Protocol)})
		e.Fill = map[string]string{"<name>": m.Project}
		return Manifest{}, e
	}
	if err := check(m); err != nil {
		return Manifest{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return m, nil
}

// check returns why a value of m does not have its form: the
// generation and the salvage id are lowercase ULIDs (03 3.12), the spec
// commit and each base are commit ids, the run digest is the digest of
// kind run of the run, the cap is not negative, and exactly one
// repository is the primary (03 3.2).
func check(m Manifest) error {
	if _, err := canon.ParseULID(m.Generation); err != nil {
		return fmt.Errorf("generation: %w", err)
	}
	if m.SalvageRun != nil {
		if _, err := canon.ParseULID(m.SalvageRun.ID); err != nil {
			return fmt.Errorf("salvageRun.id: %w", err)
		}
	}
	if !commitForm.MatchString(m.SpecCommit) {
		return fmt.Errorf("specCommit %q is not 40 lowercase hexadecimal digits", clip(m.SpecCommit))
	}
	if !digestForm.MatchString(m.RunDigest) {
		return fmt.Errorf("runDigest %q is not 64 lowercase hexadecimal digits", clip(m.RunDigest))
	}
	sum, err := canon.Digest(canon.Run, m.Run)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	if sum.String() != m.RunDigest {
		return errors.New("runDigest is not the digest of kind run of the run")
	}
	if m.Salvage.CapBytes < 0 {
		return fmt.Errorf("salvage.capBytes %d is negative", m.Salvage.CapBytes)
	}
	primaries := 0
	for i, r := range m.Repos {
		if r.Primary {
			primaries++
		}
		for _, b := range r.Base {
			if !commitForm.MatchString(b) {
				return fmt.Errorf("repos[%d].base %q is not 40 lowercase hexadecimal digits", i, clip(b))
			}
		}
	}
	if primaries != 1 {
		return fmt.Errorf("%d repositories are primary, and exactly one is", primaries)
	}
	return nil
}

// DecodeEnv reads the env value of a manifest: base64url without
// padding, at most MaxEncodedSize bytes (REQ-0215, REQ-0216), then
// Decode. Padding, another alphabet, a space and non-zero trailing
// bits are each an error.
func DecodeEnv(value string) (Manifest, error) {
	if len(value) > MaxEncodedSize {
		return Manifest{}, fmt.Errorf("%w: the env value of %d bytes passes the cap of %d", ErrInvalid, len(value), MaxEncodedSize)
	}
	// The decoder skips CR and LF; the env value holds none.
	if strings.ContainsAny(value, "\r\n") {
		return Manifest{}, fmt.Errorf("%w: the env value holds a line break", ErrInvalid)
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil {
		return Manifest{}, fmt.Errorf("%w: the env value is not base64url without padding: %w", ErrInvalid, err)
	}
	return Decode(data)
}

// Encode returns the canonical JSON of m, as 03 3.12 defines it and
// canon digests it: canon.CanonicalJSON, the one writer of that form. A
// nil list is written as [] and a nil run as {}, the forms the schema
// admits. A string or key that holds U+FFFD, or text that is not UTF-8,
// is canon.ErrInvalidText, and a number out of the float64 range
// canon.ErrNumberRange, as in canon.Digest.
func Encode(m Manifest) ([]byte, error) {
	data, err := canon.CanonicalJSON(m.withEmptyLists())
	if err != nil {
		return nil, fmt.Errorf("manifest: encode: %w", err)
	}
	return data, nil
}

// EncodeEnv returns the env value of m: the base64url, without padding,
// of its canonical JSON (REQ-0215). A value over MaxEncodedSize is
// ErrTooLarge; EncodedSize reports its size.
func EncodeEnv(m Manifest) (string, error) {
	data, err := Encode(m)
	if err != nil {
		return "", err
	}
	if n := base64.RawURLEncoding.EncodedLen(len(data)); n > MaxEncodedSize {
		return "", fmt.Errorf("%w: %d bytes, over %d", ErrTooLarge, n, MaxEncodedSize)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// EncodedSize returns the size in bytes of the env value of m, the size
// sync and julieta spec validate compare with MaxEncodedSize (03 3.6,
// REQ-0216).
func EncodedSize(m Manifest) (int, error) {
	data, err := Encode(m)
	if err != nil {
		return 0, err
	}
	return base64.RawURLEncoding.EncodedLen(len(data)), nil
}

// withEmptyLists returns a copy of m whose nil lists are empty and
// whose nil run is an empty object; m is left as it was.
func (m Manifest) withEmptyLists() Manifest {
	m.Egress = orEmpty(m.Egress)
	m.Salvage.ExcludeIgnored = orEmpty(m.Salvage.ExcludeIgnored)
	m.Repos = orEmpty(slices.Clone(m.Repos))
	for i := range m.Repos {
		m.Repos[i].Base = orEmpty(m.Repos[i].Base)
	}
	if m.Run == nil {
		m.Run = Run{}
	} else {
		m.Run = maps.Clone(m.Run)
	}
	return m
}

// orEmpty returns s, or an empty slice when s is nil.
func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// validText checks every string and key of the JSON data Decode reads
// for U+FFFD, the mark of text that is not UTF-8 (03 3.12). A duplicate
// key is left to the decoder, so that U+FFFD is always
// canon.ErrInvalidText.
func validText(data []byte) error {
	dec := jsontext.NewDecoder(bytes.NewReader(data), jsontext.AllowDuplicateNames(true))
	for {
		tok, err := dec.ReadToken()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("manifest: encode: %w", err)
		}
		if tok.Kind() == '"' && strings.ContainsRune(tok.String(), utf8.RuneError) {
			return fmt.Errorf("manifest: encode: %w: %q", canon.ErrInvalidText, clip(tok.String()))
		}
	}
}

// clipLen is the most bytes of an input that an error message quotes.
const clipLen = 64

// clip returns s cut to clipLen bytes, so that an error about a long
// input stays short.
func clip(s string) string {
	if len(s) <= clipLen {
		return s
	}
	return s[:clipLen] + "..."
}
