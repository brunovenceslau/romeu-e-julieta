// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package salvage

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
)

// MaxManifestSize is the most bytes of a manifest.json romeu reads (04
// 4.1, REQ-0271): 03 3.11 names no cap, so this one leaves room for
// some 30 000 refs and skipped items of about 250 bytes each, far more
// than a sandbox's repo holds, and keeps the decode of a manifest a
// sandbox wrote bounded.
const MaxManifestSize = 8 << 20

// ErrManifest is the error of a salvage manifest this build does not
// read: it does not decode strictly, misses a member, names another
// format or an older version, or holds a value of the wrong form. A
// newer version is format-newer (RJ-311) instead.
var ErrManifest = errors.New("salvage: not a salvage manifest")

// manifestSchema reads a schema member of the form "salvage/v<n>".
var manifestSchema = regexp.MustCompile(`^salvage/v([1-9][0-9]{0,8})$`)

// sha256Hex is a plain sha256 in lowercase hex (03 3.12, REQ-0256).
var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// DecodeManifest reads a salvage manifest strictly (03, opening, and
// 04 4.1, REQ-0271): at most MaxManifestSize bytes, a duplicate member
// name, text that is not UTF-8 and trailing data are each an error, and
// every member but agentStop must be present and not null. A member
// this build does not know is skipped, as a newer julieta of the same
// version may add one (REQ-0163). The schema is read first, so a newer
// version is format-newer (RJ-311). Then it lists at most maxPayloads
// bundles and as many files, each size is from 0 to maxPayloadSize,
// each sha256 and object id has its form, each skipped reason is one of
// the sandbox half, and agentStop, when present, counts no more killed
// than matched: present with zero matched is a --stop-agents that
// matched none with no session open (REQ-0696), absent is a salvage
// without --stop-agents. The paths are checked by
// CheckPaths and the ref names by the import, each with its own error.
// Every other refusal wraps ErrManifest.
func DecodeManifest(data []byte) (Manifest, error) {
	if len(data) > MaxManifestSize {
		return Manifest{}, fmt.Errorf("%w: %d bytes pass the cap of %d", ErrManifest, len(data), MaxManifestSize)
	}
	var head struct {
		Schema *string `json:"schema"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return Manifest{}, fmt.Errorf("%w: %w", ErrManifest, err)
	}
	if head.Schema == nil {
		return Manifest{}, fmt.Errorf("%w: no schema member", ErrManifest)
	}
	sub := manifestSchema.FindStringSubmatch(*head.Schema)
	if sub == nil {
		return Manifest{}, fmt.Errorf("%w: the schema %q is not salvage/v<n>", ErrManifest, clip(*head.Schema))
	}
	switch n, _ := strconv.Atoi(sub[1]); {
	case n > ManifestVersion:
		return Manifest{}, manifest.FormatNewer.New(fmt.Sprintf("the salvage manifest is %s, newer than %s, the version this build reads", *head.Schema, ManifestField))
	case n < ManifestVersion:
		return Manifest{}, fmt.Errorf("%w: the schema %s is older than %s", ErrManifest, *head.Schema, ManifestField)
	}
	var doc struct {
		AgentStop  *AgentStop  `json:"agentStop"`
		Bundles    *[]Bundle   `json:"bundles"`
		Complete   *bool       `json:"complete"`
		Files      *[]File     `json:"files"`
		Generation *string     `json:"generation"`
		Refs       *[]Ref      `json:"refs"`
		Repo       *string     `json:"repo"`
		SalvageID  *string     `json:"salvageId"`
		Schema     string      `json:"schema"`
		Skipped    *[]Skipped  `json:"skipped"`
		Worktrees  *[]Worktree `json:"worktrees"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return Manifest{}, fmt.Errorf("%w: %w", ErrManifest, err)
	}
	if doc.Bundles == nil || doc.Complete == nil || doc.Files == nil || doc.Generation == nil || doc.Refs == nil ||
		doc.Repo == nil || doc.SalvageID == nil || doc.Skipped == nil || doc.Worktrees == nil {
		return Manifest{}, fmt.Errorf("%w: bundles, complete, files, generation, refs, repo, salvageId, skipped and worktrees are each required", ErrManifest)
	}
	m := Manifest{
		AgentStop: doc.AgentStop, Bundles: *doc.Bundles, Complete: *doc.Complete, Files: *doc.Files,
		Generation: *doc.Generation, Refs: *doc.Refs, Repo: *doc.Repo, SalvageID: *doc.SalvageID,
		Schema: doc.Schema, Skipped: *doc.Skipped, Worktrees: *doc.Worktrees,
	}
	if err := checkManifest(m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// checkManifest returns ErrManifest for a value of m that is not of its
// form.
func checkManifest(m Manifest) error {
	if len(m.Bundles) > maxPayloads || len(m.Files) > maxPayloads {
		return fmt.Errorf("%w: %d bundles and %d files pass the cap of %d each", ErrManifest, len(m.Bundles), len(m.Files), maxPayloads)
	}
	if s := m.AgentStop; s != nil && (s.Matched < 0 || s.Killed < 0 || s.Killed > s.Matched) {
		return fmt.Errorf("%w: agentStop matched %d, killed %d", ErrManifest, s.Matched, s.Killed)
	}
	for _, b := range m.Bundles {
		if err := checkPayload(b.File, b.Size, b.SHA256); err != nil {
			return err
		}
	}
	for _, f := range m.Files {
		if err := checkPayload(f.Path, f.Size, f.SHA256); err != nil {
			return err
		}
	}
	for _, r := range m.Refs {
		if !objectID.MatchString(r.SHA) {
			return fmt.Errorf("%w: the ref %q points at %q, not a full object id", ErrManifest, clip(r.Ref), clip(r.SHA))
		}
	}
	for _, sk := range m.Skipped {
		if !slices.Contains(sandboxReasons, sk.Reason) {
			return fmt.Errorf("%w: the skipped item %q has the reason %q, not one of the sandbox half", ErrManifest, clip(sk.Item), clip(sk.Reason))
		}
	}
	for _, w := range m.Worktrees {
		if w.Head != "" && !objectID.MatchString(w.Head) {
			return fmt.Errorf("%w: the worktree %q has the HEAD %q, not a full object id", ErrManifest, clip(w.Path), clip(w.Head))
		}
	}
	return nil
}

// sandboxReasons are the skipped reasons a manifest may hold: the
// sandbox half's (03 3.11, REQ-0246). The host half's own are added on
// the host, never read from the sandbox.
var sandboxReasons = []string{
	ReasonExcludedBySpec, ReasonOverCap, ReasonTranscriptExcluded, ReasonUnreadable, ReasonDiskFull,
	ReasonWorktreeChanging, ReasonInProgressOperation, ReasonAgentNotMatched, ReasonAgentPathMissing,
}

// maxPayloadSize is the largest size a payload may declare, 2^53 bytes,
// the largest integer every JSON reader keeps exact.
const maxPayloadSize = 1 << 53

// maxPayloads is the most bundles, and the most files, one manifest
// lists: each bundle costs git processes on the host.
const maxPayloads = 4096

// checkPayload returns ErrManifest for a payload whose size is negative
// or past maxPayloadSize, or whose sha256 is not lowercase hex.
func checkPayload(p string, size int64, sum string) error {
	if size < 0 || size > maxPayloadSize || !sha256Hex.MatchString(sum) {
		return fmt.Errorf("%w: the payload %q has the size %d and the sha256 %q", ErrManifest, clip(p), size, clip(sum))
	}
	return nil
}

// CheckPaths applies the lexical half of the path rule of 03 3.11
// (REQ-0252) to every path of m: bundles[].file, files[].path,
// worktrees[].path and each embedded-repo path (bundles[].nested,
// refs[].nested, worktrees[].nested) is relative, clean, has no ".."
// element and no control character, is at most maxPathLen bytes of at
// most maxPathDepth elements, no payload is named twice, and no two
// bundles hold one repository (the same bundles[].nested). The
// filesystem half, no symbolic link and below the salvage dir, is the
// read's (openPayload). A refusal is the error salvage-path (RJ-307).
func CheckPaths(m Manifest) error {
	seen := map[string]bool{}
	payload := func(what, p string) error {
		if err := checkRel(what, p); err != nil {
			return err
		}
		if seen[p] {
			return SalvagePath.New(fmt.Sprintf("the manifest names the payload %q twice", clip(p)))
		}
		seen[p] = true
		return nil
	}
	held := map[string]bool{}
	for _, b := range m.Bundles {
		if err := payload("bundles[].file", b.File); err != nil {
			return err
		}
		if err := checkLabel("bundles[].nested", b.Nested); err != nil {
			return err
		}
		if held[b.Nested] {
			return SalvagePath.New(fmt.Sprintf("two bundles hold the repository %q", clip(b.Nested)))
		}
		held[b.Nested] = true
	}
	for _, f := range m.Files {
		if err := payload("files[].path", f.Path); err != nil {
			return err
		}
	}
	for _, r := range m.Refs {
		if err := checkLabel("refs[].nested", r.Nested); err != nil {
			return err
		}
	}
	for _, w := range m.Worktrees {
		if err := checkRel("worktrees[].path", w.Path); err != nil {
			return err
		}
		if err := checkLabel("worktrees[].nested", w.Nested); err != nil {
			return err
		}
	}
	return nil
}

// checkLabel is checkRel for an embedded-repo path, which is empty for
// the repo itself.
func checkLabel(what, p string) error {
	if p == "" {
		return nil
	}
	return checkRel(what, p)
}

// The bounds of a path of a manifest: the read of a payload judges each
// element with Lstat (checkComponents), so a deep path a sandbox made
// costs no more than these.
const (
	maxPathLen   = 4096
	maxPathDepth = 64
)

// checkRel returns salvage-path (RJ-307) for a path p of the manifest
// member what that is not relative and clean, that holds a ".."
// element, an empty or "." one, or a control or format character, or
// that passes maxPathLen or maxPathDepth.
func checkRel(what, p string) error {
	switch {
	case len(p) > maxPathLen || strings.Count(p, "/") >= maxPathDepth:
		return SalvagePath.New(fmt.Sprintf("%s %q is longer or deeper than %d bytes and %d elements", what, clip(p), maxPathLen, maxPathDepth))
	case p == "" || p == ".":
		return SalvagePath.New(fmt.Sprintf("%s %q is empty", what, clip(p)))
	case strings.HasPrefix(p, "/"):
		return SalvagePath.New(fmt.Sprintf("%s %q is absolute", what, clip(p)))
	case path.Clean(p) != p:
		return SalvagePath.New(fmt.Sprintf("%s %q is not clean", what, clip(p)))
	case strings.ContainsFunc(p, isHostile):
		return SalvagePath.New(fmt.Sprintf("%s %q holds a control or format character", what, clip(p)))
	}
	for e := range strings.SplitSeq(p, "/") {
		if e == ".." {
			return SalvagePath.New(fmt.Sprintf("%s %q holds a .. element", what, clip(p)))
		}
	}
	return nil
}

// isHostile reports whether r is a control or a format character, or a
// line or paragraph separator, which a terminal could be made to
// misread (I29).
func isHostile(r rune) bool {
	return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == 0x2028 || r == 0x2029
}
