// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

// sum64 is a sha256 of the form of a manifest.
var sum64 = strings.Repeat("ab", 32)

// oid40 is a full object id.
var oid40 = strings.Repeat("c", 40)

// validManifest returns a manifest that decodes.
func validManifest() salvage.Manifest {
	return salvage.Manifest{
		Bundles:    []salvage.Bundle{{File: "repo.bundle", SHA256: sum64, Size: 10}, {File: "nested-1.bundle", Nested: "worktree-0/vendor/x", SHA256: sum64, Size: 0}},
		Files:      []salvage.File{{Path: "ignored.tar.gz", SHA256: sum64, Size: 3}, {Path: "agent/claude.tar.gz", SHA256: sum64, Size: 4}},
		Generation: "01j9zc2q8x7w4t6y3r5e1v9k0p",
		Refs:       []salvage.Ref{{Ref: "refs/heads/main", SHA: oid40}, {Nested: "worktree-0/vendor/x", Ref: "refs/heads/main", SHA: oid40}},
		Repo:       "api",
		SalvageID:  "01jb000000000000000000000a",
		Skipped:    []salvage.Skipped{{Item: "worktree-0/big", Reason: salvage.ReasonOverCap}},
		Worktrees:  []salvage.Worktree{{Path: "worktree-0", Head: oid40, Location: "/home/agent/api"}, {Path: "worktree-0/vendor/x", Nested: "worktree-0/vendor/x", Location: "/x"}},
	}
}

// encode returns the bytes of m.
func encode(t *testing.T, m salvage.Manifest) []byte {
	t.Helper()
	data, err := salvage.EncodeManifest(m)
	require.NoError(t, err)
	return data
}

// TestDecodeManifest decodes what EncodeManifest writes, and reads
// agentStop as 03 3.11 defines it: absent without --stop-agents, and
// present with zero matched for a --stop-agents that matched none with
// no session open.
func TestDecodeManifest(t *testing.T) {
	m := validManifest()
	got, err := salvage.DecodeManifest(encode(t, m))
	require.NoError(t, err)
	assert.Nil(t, got.AgentStop)
	assert.Equal(t, m.Refs, got.Refs)
	assert.False(t, got.Complete)

	m.AgentStop = &salvage.AgentStop{}
	got, err = salvage.DecodeManifest(encode(t, m))
	require.NoError(t, err)
	require.NotNil(t, got.AgentStop)
	assert.Equal(t, salvage.AgentStop{Matched: 0, Killed: 0}, *got.AgentStop)
}

// TestDecodeManifestUnknownMember skips a member this build does not
// know (REQ-0163).
func TestDecodeManifestUnknownMember(t *testing.T) {
	data := bytes.Replace(encode(t, validManifest()), []byte(`{`), []byte(`{"later":{"a":1},`), 1)
	_, err := salvage.DecodeManifest(data)
	require.NoError(t, err)
}

// TestDecodeManifestRefuses refuses each document that is not a
// salvage.v1 manifest of the form, and a newer version as format-newer.
func TestDecodeManifestRefuses(t *testing.T) {
	valid := string(encode(t, validManifest()))
	edit := func(f func(*salvage.Manifest)) string {
		m := validManifest()
		f(&m)
		return string(encode(t, m))
	}
	tests := []struct {
		name string
		data string
	}{
		{"not JSON", "{"},
		{"no schema", `{"repo":"api"}`},
		{"another format", strings.Replace(valid, `"salvage/v1"`, `"snapshot/v1"`, 1)},
		{"an older version", strings.Replace(valid, `"salvage/v1"`, `"salvage/v0"`, 1)},
		{"a missing member", strings.Replace(valid, `"refs":`, `"refz":`, 1)},
		{"a null member", strings.Replace(valid, `"repo":"api"`, `"repo":null`, 1)},
		{"a duplicate member", strings.Replace(valid, `"repo":"api"`, `"repo":"api","repo":"web"`, 1)},
		{"trailing data", valid + "{}"},
		{"a negative size", edit(func(m *salvage.Manifest) { m.Files[0].Size = -1 })},
		{"a short sha256", edit(func(m *salvage.Manifest) { m.Bundles[0].SHA256 = "ab" })},
		{"an uppercase sha256", edit(func(m *salvage.Manifest) { m.Files[0].SHA256 = strings.ToUpper(sum64) })},
		{"a short object id", edit(func(m *salvage.Manifest) { m.Refs[0].SHA = "abc" })},
		{"a worktree HEAD of another form", edit(func(m *salvage.Manifest) { m.Worktrees[0].Head = "HEAD" })},
		{"a host reason", edit(func(m *salvage.Manifest) { m.Skipped[0].Reason = salvage.ReasonPayloadMismatch })},
		{"an unknown reason", edit(func(m *salvage.Manifest) { m.Skipped[0].Reason = "fine" })},
		{"a size past 2^53", edit(func(m *salvage.Manifest) { m.Files[0].Size = 1<<53 + 1 })},
		{"too many bundles", edit(func(m *salvage.Manifest) {
			for range 4096 {
				m.Bundles = append(m.Bundles, m.Bundles[0])
			}
		})},
		{"more killed than matched", edit(func(m *salvage.Manifest) { m.AgentStop = &salvage.AgentStop{Matched: 1, Killed: 2} })},
		{"over the cap", valid[:len(valid)-2] + `,"pad":"` + strings.Repeat("x", salvage.MaxManifestSize) + `"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := salvage.DecodeManifest([]byte(tt.data))
			require.ErrorIs(t, err, salvage.ErrManifest)
		})
	}

	_, err := salvage.DecodeManifest([]byte(strings.Replace(valid, `"salvage/v1"`, `"salvage/v2"`, 1)))
	e, ok := errors.AsType[*cli.Error](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, manifest.FormatNewer, e.Row)
}

// TestCheckPaths refuses each hostile path of each member REQ-0252
// names as salvage-path (RJ-307), and passes the paths julieta writes.
func TestCheckPaths(t *testing.T) {
	require.NoError(t, salvage.CheckPaths(validManifest()))
	hostile := []struct {
		name, path string
	}{
		{"empty", ""},
		{"dot", "."},
		{"absolute", "/etc/passwd"},
		{"dot-dot", ".."},
		{"a dot-dot element", "agent/../../x"},
		{"a leading dot-dot", "../x"},
		{"not clean", "agent//x"},
		{"a trailing slash", "agent/"},
		{"a dot element", "./x"},
		{"a control character", "a\x1b[2Jb"},
		{"a format character", "a\u202eb"},
	}
	members := []struct {
		name string
		set  func(*salvage.Manifest, string)
	}{
		{"bundles[].file", func(m *salvage.Manifest, p string) { m.Bundles[0].File = p }},
		{"bundles[].nested", func(m *salvage.Manifest, p string) { m.Bundles[1].Nested = p }},
		{"files[].path", func(m *salvage.Manifest, p string) { m.Files[1].Path = p }},
		{"refs[].nested", func(m *salvage.Manifest, p string) { m.Refs[1].Nested = p }},
		{"worktrees[].path", func(m *salvage.Manifest, p string) { m.Worktrees[0].Path = p }},
		{"worktrees[].nested", func(m *salvage.Manifest, p string) { m.Worktrees[1].Nested = p }},
	}
	for _, mb := range members {
		for _, h := range hostile {
			if h.path == "" && strings.HasSuffix(mb.name, ".nested") {
				continue // an empty embedded-repo path is the repo itself
			}
			t.Run(mb.name+"/"+h.name, func(t *testing.T) {
				m := validManifest()
				mb.set(&m, h.path)
				assertRow(t, salvage.SalvagePath, salvage.CheckPaths(m))
			})
		}
	}
	t.Run("a path too deep", func(t *testing.T) {
		m := validManifest()
		m.Files[0].Path = strings.Repeat("a/", 64) + "f"
		assertRow(t, salvage.SalvagePath, salvage.CheckPaths(m))
	})
	t.Run("a path too long", func(t *testing.T) {
		m := validManifest()
		m.Files[0].Path = strings.Repeat("a", 4097)
		assertRow(t, salvage.SalvagePath, salvage.CheckPaths(m))
	})
	t.Run("two bundles of one repository", func(t *testing.T) {
		m := validManifest()
		m.Bundles[1].Nested = ""
		assertRow(t, salvage.SalvagePath, salvage.CheckPaths(m))
	})
	t.Run("a payload named twice", func(t *testing.T) {
		m := validManifest()
		m.Files[0].Path = m.Bundles[0].File
		assertRow(t, salvage.SalvagePath, salvage.CheckPaths(m))
	})
}

// assertRow asserts that err is an error of the table of row r.
func assertRow(t *testing.T, r cli.Row, err error) {
	t.Helper()
	e, ok := errors.AsType[*cli.Error](err)
	require.True(t, ok, "not an error of the table: %v", err)
	assert.Equal(t, r, e.Row, "%v", err)
}

// TestCompleteAndReasons recomputes completeness from the items alone:
// excluded-by-spec and transcript-excluded leave a salvage complete
// (REQ-0717), and the reasons list each other reason once, sorted.
func TestCompleteAndReasons(t *testing.T) {
	accepted := []salvage.Item{{Reason: salvage.ReasonExcludedBySpec}, {Reason: salvage.ReasonTranscriptExcluded}}
	assert.True(t, salvage.Complete(nil))
	assert.True(t, salvage.Complete(accepted))
	assert.Empty(t, salvage.Reasons(accepted))
	items := append(accepted, salvage.Item{Reason: salvage.ReasonRepoMissing}, salvage.Item{Reason: salvage.ReasonFsckFailed}, salvage.Item{Reason: salvage.ReasonRepoMissing})
	assert.False(t, salvage.Complete(items))
	assert.Equal(t, []string{salvage.ReasonFsckFailed, salvage.ReasonRepoMissing}, salvage.Reasons(items))
}
