// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memory_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/memory"
)

// line40 is a line of exactly 40 bytes, the shortest the check compares.
const line40 = "Run the local CI before asking review ok"

// writeRepo writes the files of the primary repo of s, by name relative
// to it.
func writeRepo(t *testing.T, s *sandbox, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(s.m.Repos[0].Path, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	}
}

// derivableLines returns the memory-derivable lines of stderr.
func derivableLines(stderr string) []string {
	var out []string
	for l := range strings.Lines(stderr) {
		if strings.HasPrefix(l, "warning memory-derivable: ") {
			out = append(out, strings.TrimSuffix(l, "\n"))
		}
	}
	return out
}

// TestDerivableWarns warns memory-derivable, exit 0 and the entry
// written and listed, when a body line of 40 bytes equals a CLAUDE.md
// line with other surrounding white space, naming the file and the line
// (REQ-1290); a 39-byte match warns nothing.
func TestDerivableWarns(t *testing.T) {
	require.Len(t, line40, 40)
	s := newSandbox(t)
	writeRepo(t, s, map[string]string{"CLAUDE.md": "# Rules\n\n  " + line40 + "\t\n" + line40[:39] + "\n"})
	code, stdout, stderr := s.julieta(t, "memory", "add", "--title", "t", "--tag", "feedback", "--body", "Learned:\n"+line40+"   \n")
	require.Equal(t, 0, code, stderr)
	assert.True(t, strings.HasPrefix(stdout, "added "), stdout)
	assert.Equal(t, []string{"warning memory-derivable: CLAUDE.md:3 holds a line of the body; keep out what an instruction file already holds"},
		derivableLines(stderr))
	assert.Len(t, s.report(t, "memory", "list").Entries, 1)

	code, _, stderr = s.julieta(t, "memory", "add", "--title", "u", "--tag", "feedback", "--body", line40[:39])
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, derivableLines(stderr), "a 39-byte match")
}

// TestDerivableImportsAndLinks follows an @ import of AGENTS.md from
// inside the repo and names the imported file; an import of /etc/passwd,
// of a missing file or out of the repo, and a rule file that is a
// symlink out of the repo, are skipped (REQ-1290).
func TestDerivableImportsAndLinks(t *testing.T) {
	s := newSandbox(t)
	outside := filepath.Join(t.TempDir(), "out.md")
	require.NoError(t, os.WriteFile(outside, []byte(line40+"\n"), 0o600))
	writeRepo(t, s, map[string]string{
		"AGENTS.md":          "@docs/rules.md\n@/etc/passwd\n@docs/missing.md\n@../out.md\n",
		"docs/rules.md":      "x\n" + line40 + "\n",
		".claude/rules/a.md": "nothing here\n",
	})
	require.NoError(t, os.Symlink(outside, filepath.Join(s.m.Repos[0].Path, ".claude", "rules", "x.md")))
	code, _, stderr := s.julieta(t, "memory", "add", "--title", "t", "--tag", "project", "--body", line40+"\nroot:x:0:0:root:/root:/bin/sh and more text\n")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, []string{"warning memory-derivable: docs/rules.md:2 holds a line of the body; keep out what an instruction file already holds"},
		derivableLines(stderr))
}

// TestDerivableBounds prints ten matches and then "1 more" for eleven,
// warns on a duplicate of an open entry too, which writes nothing, keeps
// the --json document as it is, and names a CLAUDE.md of 1 MiB plus one
// byte in place of comparing it (REQ-1290).
func TestDerivableBounds(t *testing.T) {
	s := newSandbox(t)
	var rules, body strings.Builder
	for i := range 11 {
		l := fmt.Sprintf("Rule number %02d of the instruction file here", i)
		rules.WriteString(l + "\n")
		body.WriteString(l + "\n")
	}
	writeRepo(t, s, map[string]string{"AGENTS.md": rules.String(), "CLAUDE.md": strings.Repeat("x", 1<<20+1)})
	args := []string{"memory", "add", "--title", "t", "--tag", "project", "--body", body.String(), "--json"}
	code, stdout, stderr := s.julieta(t, args...)
	require.Equal(t, 0, code, stderr)
	rep, err := memory.Decode([]byte(stdout))
	require.NoError(t, err, stdout)
	assert.True(t, rep.Wrote)
	got := derivableLines(stderr)
	require.Len(t, got, 12, stderr)
	assert.Equal(t, "warning memory-derivable: AGENTS.md:1 holds a line of the body; keep out what an instruction file already holds", got[0])
	assert.Equal(t, "warning memory-derivable: 1 more", got[10])
	assert.Equal(t, "warning memory-derivable: CLAUDE.md is over 1048576 bytes and was not compared", got[11])

	code, stdout, stderr = s.julieta(t, args...)
	require.Equal(t, 0, code, stderr)
	again, err := memory.Decode([]byte(stdout))
	require.NoError(t, err, stdout)
	assert.False(t, again.Wrote, "the duplicate writes nothing")
	assert.Len(t, derivableLines(stderr), 12, "and warns too")
}

// TestDerivableWithoutFiles warns nothing in a repo with no instruction
// file.
func TestDerivableWithoutFiles(t *testing.T) {
	s := newSandbox(t)
	code, _, stderr := s.julieta(t, "memory", "add", "--title", "t", "--tag", "user", "--body", line40)
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, stderr)
}

// TestDerivableEdges counts bytes, not runes: a 40-byte line of 20 runes
// warns and a 39-byte one does not; an import is followed one level
// only; an import through an in-repo symlink out of the repo is skipped;
// and a CLAUDE.md or a .claude/rules that is a FIFO neither blocks add
// nor is read (REQ-1290).
func TestDerivableEdges(t *testing.T) {
	s := newSandbox(t)
	wide, short := strings.Repeat("é", 20), strings.Repeat("é", 19)+"a"
	outside := filepath.Join(t.TempDir(), "out.md")
	require.NoError(t, os.WriteFile(outside, []byte(line40+"\n"), 0o600))
	writeRepo(t, s, map[string]string{
		"CLAUDE.md":  wide + "\n" + short + "\n@a.md\n@docs/l.md\n",
		"a.md":       "@b.md\n",
		"b.md":       line40 + "\n",
		"docs/x.txt": "",
	})
	repo := s.m.Repos[0].Path
	require.NoError(t, os.Symlink(outside, filepath.Join(repo, "docs", "l.md")))
	require.NoError(t, syscall.Mkfifo(filepath.Join(repo, "AGENTS.md"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".claude"), 0o700))
	require.NoError(t, syscall.Mkfifo(filepath.Join(repo, ".claude", "rules"), 0o600))
	done := make(chan []string, 1)
	go func() {
		code, _, stderr := s.julieta(t, "memory", "add", "--title", "t", "--tag", "project", "--body", wide+"\n"+short+"\n"+line40+"\n")
		assert.Equal(t, 0, code, stderr)
		done <- derivableLines(stderr)
	}()
	select {
	case got := <-done:
		assert.Equal(t, []string{"warning memory-derivable: CLAUDE.md:1 holds a line of the body; keep out what an instruction file already holds"}, got)
	case <-time.After(10 * time.Second):
		t.Fatal("memory add blocked on a FIFO")
	}
}
