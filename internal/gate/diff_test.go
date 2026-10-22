// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

var update = flag.Bool("update", false, "rewrite the golden files of testdata")

// golden compares got with testdata/name, or rewrites it with -update.
func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	p := filepath.Join("testdata", name)
	if *update {
		require.NoError(t, os.WriteFile(p, got, 0o644))
	}
	want, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}

// side returns the diff side of in: its widening set, its kit files and
// its secret argv.
func side(t *testing.T, in inputs) Side {
	t.Helper()
	s, err := Build(in.p, in.hs, in.r)
	require.NoError(t, err)
	out := Side{Set: s, KitFiles: map[string][]File{}, SecretArgv: map[string][]string{}}
	for i, k := range s.Kits {
		out.KitFiles[k.key()] = in.r.Kits[i].Files
	}
	for _, name := range in.p.Secrets {
		out.SecretArgv[name] = in.hs.Secrets[name+"@"+in.p.Name].Argv
	}
	return out
}

// text returns what the diff from approved to candidate writes.
func text(t *testing.T, approved *Side, candidate Side) string {
	t.Helper()
	d, err := NewDiff(approved, candidate)
	require.NoError(t, err)
	var b strings.Builder
	_, err = d.WriteTo(&b)
	require.NoError(t, err)
	return b.String()
}

// TestDiffGolden changes one item of each class of 01 1.4: a
// capability, a gated domain, a secret argv, the workload repository
// and a content-only kit file (10 10.1, REQ-0732).
//
//romeu:invariant I29 test
func TestDiffGolden(t *testing.T) {
	t.Parallel()
	old := baseInputs()
	old.r.Kits[1].Files = append(old.r.Kits[1].Files, File{Path: "README", Mode: ModeRegular, Data: []byte("one\ntwo\nthree\n")})
	cand := baseInputs()
	cand.r.Kits[1].Files = append(cand.r.Kits[1].Files, File{Path: "README", Mode: ModeRegular, Data: []byte("one\n2\nthree\n")})
	cand.r.WorkloadCapabilities = Capabilities{"network": []any{"registry.example"}, "mounts": []any{"/var/run/docker.sock"}}
	cand.p.Egress.Extra = append(cand.p.Egress.Extra, "upload.example")
	cand.hs.Secrets["github@shop"] = spec.Secret{Service: "github.com", Argv: []string{"/usr/bin/env", "-i", "/usr/local/bin/gh", "auth", "token"}}
	cand.p.Workload = "docker.io/other/dev@sha256:" + hex64('a')

	approved := side(t, old)
	golden(t, "diff.golden", []byte(text(t, &approved, side(t, cand))))
}

func TestDiffGoldenOfAFirstApproval(t *testing.T) {
	t.Parallel()
	golden(t, "diff-first.golden", []byte(text(t, nil, side(t, baseInputs()))))
}

// TestDiffIsNeverTruncated changes every line of a 10 000-line kit
// file: each line appears, removed and added, and the summary is
// repeated right above the prompt (I29, REQ-0071).
//
//romeu:invariant I29 test
func TestDiffIsNeverTruncated(t *testing.T) {
	t.Parallel()
	var oldText, newText strings.Builder
	for i := range 10000 {
		fmt.Fprintf(&oldText, "line %d\n", i)
		fmt.Fprintf(&newText, "LINE %d\n", i)
	}
	old, cand := baseInputs(), baseInputs()
	old.r.Kits[1].Files[0].Data = []byte(oldText.String())
	cand.r.Kits[1].Files[0].Data = []byte(newText.String())
	approved := side(t, old)
	got := text(t, &approved, side(t, cand))

	lines := map[string]bool{}
	for l := range strings.SplitSeq(got, "\n") {
		lines[l] = true
	}
	for i := range 10000 {
		require.True(t, lines[fmt.Sprintf("      -line %d", i)], "the removed line %d", i)
		require.True(t, lines[fmt.Sprintf("      +LINE %d", i)], "the added line %d", i)
	}
	summary := "  kit content personal:tools: 1 file(s) changed, 0 added, 0 removed; +10000 -10000 lines\n" +
		"  changed: kits\n"
	assert.True(t, strings.HasPrefix(got[strings.Index(got, "summary:\n"):], "summary:\n"+summary), "the summary comes first")
	assert.True(t, strings.HasSuffix(got, summary+PromptLine), "the summary is repeated right above the prompt")
}

func TestDiffShowsABinaryFileAsSizeAndSHA256(t *testing.T) {
	t.Parallel()
	old, cand := baseInputs(), baseInputs()
	old.r.Kits[1].Files[0].Data = []byte{0x7f, 'E', 'L', 'F', 0, 1}
	cand.r.Kits[1].Files[0].Data = []byte{0xff, 0xfe, 'x'}
	approved := side(t, old)
	got := text(t, &approved, side(t, cand))
	oldSum, newSum := sha256.Sum256(old.r.Kits[1].Files[0].Data), sha256.Sum256(cand.r.Kits[1].Files[0].Data)
	assert.Contains(t, got, "      - binary, 6 bytes, sha256 "+hex.EncodeToString(oldSum[:])+"\n")
	assert.Contains(t, got, "      + binary, 3 bytes, sha256 "+hex.EncodeToString(newSum[:])+"\n")
	assert.NotContains(t, got, "ELF")
}

func TestDiffEscapesWhatAKitFileHolds(t *testing.T) {
	t.Parallel()
	old, cand := baseInputs(), baseInputs()
	cand.r.Kits[1].Files[0].Data = []byte("echo \x1b[2Jcleared\u202e\n")
	approved := side(t, old)
	got := text(t, &approved, side(t, cand))
	assert.NotContains(t, got, "\x1b")
	assert.NotContains(t, got, "\u202e")
	assert.Contains(t, got, "+echo \\u001b[2Jcleared\\u202e")
}

func TestDiffShowsAChangeOfTheLastNewlineAlone(t *testing.T) {
	t.Parallel()
	old, cand := baseInputs(), baseInputs()
	cand.r.Kits[1].Files[0].Data = []byte("#!/bin/sh")
	approved := side(t, old)
	got := text(t, &approved, side(t, cand))
	assert.Contains(t, got, "      -#!/bin/sh\n      +#!/bin/sh (no newline at end of file)\n")
}

func TestNewDiffRefusesFilesOrArgvTheSetDoesNotDigest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		edit func(s *Side)
	}{
		{"a kit file the set does not digest", func(s *Side) {
			s.KitFiles["personal:tools"] = []File{{Path: "bin/run", Mode: ModeExecutable, Data: []byte("other\n")}}
		}},
		{"a kit with no files", func(s *Side) { delete(s.KitFiles, "personal:tools") }},
		{"files of a kit the set does not hold", func(s *Side) { s.KitFiles["personal:other"] = nil }},
		{"an argv the set does not digest", func(s *Side) { s.SecretArgv["github"] = []string{"/bin/true"} }},
		{"a secret with no argv", func(s *Side) { delete(s.SecretArgv, "github") }},
		{"an argv of a secret the set does not hold", func(s *Side) { s.SecretArgv["npm"] = []string{"/bin/true"} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			approved := side(t, baseInputs())
			cand := side(t, baseInputs())
			tt.edit(&cand)
			_, err := NewDiff(&approved, cand)
			require.Error(t, err, "the candidate")
			_, err = NewDiff(nil, cand)
			require.Error(t, err, "a first approval")
		})
	}
}

func TestNewDiffRefusesApprovedKitFilesTheApprovalDoesNotDigest(t *testing.T) {
	t.Parallel()
	approved := side(t, baseInputs())
	approved.KitFiles["personal:tools"] = []File{{Path: "bin/run", Mode: ModeExecutable, Data: []byte("other\n")}}
	_, err := NewDiff(&approved, side(t, baseInputs()))
	assert.Error(t, err)
}
