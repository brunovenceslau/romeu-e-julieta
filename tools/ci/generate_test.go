// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// generateFixture is a repository with the sources of generate: the
// ask-first list, .adr-dir and two records.
func generateFixture(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	r.Write(askFirstPath, validAskFirst)
	r.Write(".adr-dir", "docs/adr\n")
	r.Write("docs/adr/0001-first.md", "# 1. First\n\n## Status\n\nAccepted\n")
	r.Write("docs/adr/0002-second.md", "# 2. Second\n\n## Status\n\nProposed\n")
	return r
}

func runGen(t *testing.T, r *gittest.Repo, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	e := env{dir: r.Dir, gitEnv: r.Env, stdout: &out, stderr: &errOut}
	return run(context.Background(), e, append([]string{"generate"}, args...)), out.String(), errOut.String()
}

// TestGenerateWritesTheConsumers is the acceptance case: from the
// ask-first list and the titles and statuses of the records, generate
// writes CODEOWNERS, the reference page and the index.
func TestGenerateWritesTheConsumers(t *testing.T) {
	r := generateFixture(t)
	code, out, errOut := runGen(t, r)
	require.Equal(t, exitOK, code, errOut)
	assert.Equal(t, "wrote .github/CODEOWNERS\nwrote docs/reference/ask-first.md\nwrote docs/adr/README.md\n", out)
	read := func(p string) string {
		b, err := os.ReadFile(filepath.Join(r.Dir, filepath.FromSlash(p)))
		require.NoError(t, err)
		return string(b)
	}
	assert.Contains(t, read(".github/CODEOWNERS"), "/.github/ask-first.yaml @someone\n")
	assert.Contains(t, read("docs/reference/ask-first.md"), "| `ask-first` |")
	index := read("docs/adr/README.md")
	assert.Contains(t, index, "| 1 | [First](0001-first.md) | Accepted |\n")
	assert.Contains(t, index, "| 2 | [Second](0002-second.md) | Proposed |\n")
}

// TestGenerateIsIdempotent checks that a second run writes nothing and
// leaves the modification times alone.
func TestGenerateIsIdempotent(t *testing.T) {
	r := generateFixture(t)
	code, _, errOut := runGen(t, r)
	require.Equal(t, exitOK, code, errOut)
	path := filepath.Join(r.Dir, ".github", "CODEOWNERS")
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(path, old, old))
	code, out, errOut := runGen(t, r)
	require.Equal(t, exitOK, code, errOut)
	assert.Empty(t, out)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, info.ModTime().Equal(old), "a current file is not rewritten")
}

// TestGenerateRewritesAHandEdit checks that a hand-edited consumer is
// put back, which is what makes "generated" fail on it.
func TestGenerateRewritesAHandEdit(t *testing.T) {
	r := generateFixture(t)
	runGen(t, r)
	r.Write("docs/adr/README.md", "edited\n")
	code, out, _ := runGen(t, r)
	require.Equal(t, exitOK, code)
	assert.Equal(t, "wrote docs/adr/README.md\n", out)
}

// TestGenerateRefuses covers each source that cannot be read: it exits 2,
// says why, and writes no consumer.
func TestGenerateRefuses(t *testing.T) {
	tests := []struct {
		name  string
		setup func(r *gittest.Repo)
		args  []string
		want  string
	}{
		{"an argument", nil, []string{"x"}, "generate takes no argument"},
		{"no ask-first list", func(r *gittest.Repo) { require.NoError(t, os.Remove(filepath.Join(r.Dir, askFirstPath))) }, nil, "ask-first.yaml"},
		{"an invalid list", func(r *gittest.Repo) { r.Write(askFirstPath, "version: 3\n") }, nil, "version is 3"},
		{"no .adr-dir", func(r *gittest.Repo) { require.NoError(t, os.Remove(filepath.Join(r.Dir, ".adr-dir"))) }, nil, ".adr-dir"},
		{"an .adr-dir outside the tree", func(r *gittest.Repo) { r.Write(".adr-dir", "/etc\n") }, nil, "not a directory inside the repository"},
		{"a missing record directory", func(r *gittest.Repo) { r.Write(".adr-dir", "nowhere\n") }, nil, "nowhere"},
		{"a record with a bad status", func(r *gittest.Repo) { r.Write("docs/adr/0003-third.md", "# 3. Third\n\n## Status\n\nDraft\n") }, nil, "0003-third.md"},
		{"not a repository", func(r *gittest.Repo) { require.NoError(t, os.RemoveAll(filepath.Join(r.Dir, ".git"))) }, nil, "git rev-parse"},
		{"an .adr-dir that links outside the tree", func(r *gittest.Repo) {
			outside := t.TempDir()
			require.NoError(t, os.Symlink(outside, filepath.Join(r.Dir, "out")))
			r.Write(".adr-dir", "out\n")
		}, nil, "out"},
		{"an ask-first list that links outside the tree", func(r *gittest.Repo) {
			outside := filepath.Join(t.TempDir(), "list.yaml")
			require.NoError(t, os.WriteFile(outside, []byte(validAskFirst), 0o644))
			require.NoError(t, os.Remove(filepath.Join(r.Dir, askFirstPath)))
			require.NoError(t, os.Symlink(outside, filepath.Join(r.Dir, askFirstPath)))
		}, nil, "ask-first.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := generateFixture(t)
			if tt.setup != nil {
				tt.setup(r)
			}
			code, out, errOut := runGen(t, r, tt.args...)
			assert.Equal(t, exitError, code)
			assert.Empty(t, out)
			assert.Contains(t, errOut, tt.want)
			assert.NoFileExists(t, filepath.Join(r.Dir, ".github", "CODEOWNERS"))
		})
	}
}

// TestGenerateSkipsWhatIsNoRecord checks that the index lists the
// records and nothing else: not a README, not a directory with a
// record's name.
func TestGenerateSkipsWhatIsNoRecord(t *testing.T) {
	r := generateFixture(t)
	r.Write("docs/adr/README.md", "old\n")
	require.NoError(t, os.Mkdir(filepath.Join(r.Dir, "docs", "adr", "0009-dir.md"), 0o755))
	code, _, errOut := runGen(t, r)
	require.Equal(t, exitOK, code, errOut)
	b, err := os.ReadFile(filepath.Join(r.Dir, "docs", "adr", "README.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(b), "0009")
	assert.Contains(t, string(b), "0002-second.md")
}

// rootOf opens dir as a root for the length of the test.
func rootOf(t *testing.T, dir string) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })
	return root
}

// TestWriteIfChangedReportsAnUnwritableTarget checks the error paths of
// the writer that the fixtures above cannot reach.
func TestWriteIfChangedReportsAnUnwritableTarget(t *testing.T) {
	dir := t.TempDir()
	root := rootOf(t, dir)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "d"), 0o755))
	_, err := writeIfChanged(root, "d", []byte("x"))
	require.Error(t, err, "a directory is not a file to write")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "the temporary file of the failed write is gone")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f"), nil, 0o644))
	_, err = writeIfChanged(root, "f/child", []byte("x"))
	require.Error(t, err, "a file is not a directory to create in")
}

// TestWriteIfChangedStaysInsideTheRoot checks that a symbolic link at the
// target, or in its directory, never sends a write outside the root: the
// link to a file outside is replaced, and the link to a directory
// outside is an error that writes nothing there.
func TestWriteIfChangedStaysInsideTheRoot(t *testing.T) {
	t.Run("a link to a file outside is replaced", func(t *testing.T) {
		dir, outside := t.TempDir(), filepath.Join(t.TempDir(), "victim")
		require.NoError(t, os.WriteFile(outside, []byte("mine\n"), 0o644))
		require.NoError(t, os.Symlink(outside, filepath.Join(dir, "out.md")))
		wrote, err := writeIfChanged(rootOf(t, dir), "out.md", []byte("generated\n"))
		require.NoError(t, err)
		assert.True(t, wrote)
		info, err := os.Lstat(filepath.Join(dir, "out.md"))
		require.NoError(t, err)
		assert.True(t, info.Mode().IsRegular(), "a regular file now")
		b, err := os.ReadFile(outside)
		require.NoError(t, err)
		assert.Equal(t, "mine\n", string(b), "the file outside is untouched")
	})
	t.Run("a hard link to a file outside is replaced, not written through", func(t *testing.T) {
		dir, outsideDir := t.TempDir(), t.TempDir()
		outside := filepath.Join(outsideDir, "victim")
		require.NoError(t, os.WriteFile(outside, []byte("mine\n"), 0o644))
		require.NoError(t, os.Link(outside, filepath.Join(dir, "out.md")))
		wrote, err := writeIfChanged(rootOf(t, dir), "out.md", []byte("generated\n"))
		require.NoError(t, err)
		assert.True(t, wrote)
		b, err := os.ReadFile(outside)
		require.NoError(t, err)
		assert.Equal(t, "mine\n", string(b), "the other name of the file keeps its bytes")
		b, err = os.ReadFile(filepath.Join(dir, "out.md"))
		require.NoError(t, err)
		assert.Equal(t, "generated\n", string(b))
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Len(t, entries, 1, "no temporary file is left")
	})
	t.Run("a link to a directory outside is an error", func(t *testing.T) {
		dir, outside := t.TempDir(), t.TempDir()
		require.NoError(t, os.Symlink(outside, filepath.Join(dir, "sub")))
		_, err := writeIfChanged(rootOf(t, dir), "sub/out.md", []byte("generated\n"))
		require.Error(t, err)
		assert.NoFileExists(t, filepath.Join(outside, "out.md"))
	})
	t.Run("a link to an identical copy inside is replaced", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "copy"), []byte("generated\n"), 0o644))
		require.NoError(t, os.Symlink("copy", filepath.Join(dir, "out.md")))
		wrote, err := writeIfChanged(rootOf(t, dir), "out.md", []byte("generated\n"))
		require.NoError(t, err)
		assert.True(t, wrote, "the link is not current, whatever it reads")
		info, err := os.Lstat(filepath.Join(dir, "out.md"))
		require.NoError(t, err)
		assert.True(t, info.Mode().IsRegular())
	})
}

// TestWriteIfChangedDropsTheExecuteBit checks that a generated file with
// the right bytes and an execute bit is put back to 0644, since git
// tracks the bit.
func TestWriteIfChangedDropsTheExecuteBit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o755))
	require.NoError(t, os.Chmod(path, 0o755))
	wrote, err := writeIfChanged(rootOf(t, dir), "f", []byte("x"))
	require.NoError(t, err)
	assert.True(t, wrote)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

// TestGenerateWritesNothingOutsideTheTree is the case of a generated
// page committed as a link to a path outside the repository: generate
// replaces the link, and the file it pointed to is untouched.
func TestGenerateWritesNothingOutsideTheTree(t *testing.T) {
	r := generateFixture(t)
	victim := filepath.Join(t.TempDir(), "victim")
	require.NoError(t, os.WriteFile(victim, []byte("mine\n"), 0o644))
	page := filepath.Join(r.Dir, filepath.FromSlash(askFirstPagePath))
	require.NoError(t, os.MkdirAll(filepath.Dir(page), 0o755))
	require.NoError(t, os.Symlink(victim, page))
	code, _, errOut := runGen(t, r)
	require.Equal(t, exitOK, code, errOut)
	b, err := os.ReadFile(victim)
	require.NoError(t, err)
	assert.Equal(t, "mine\n", string(b))
	info, err := os.Lstat(page)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular())
}

// TestGenerateLeavesARecordLinkOutOfTheIndex checks that a symbolic link
// with a record's name is not read: its target may lie anywhere.
func TestGenerateLeavesARecordLinkOutOfTheIndex(t *testing.T) {
	r := generateFixture(t)
	outside := filepath.Join(t.TempDir(), "0003-link.md")
	require.NoError(t, os.WriteFile(outside, []byte("# 3. Link\n\n## Status\n\nAccepted\n"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(r.Dir, "docs", "adr", "0003-link.md")))
	code, _, errOut := runGen(t, r)
	require.Equal(t, exitOK, code, errOut)
	b, err := os.ReadFile(filepath.Join(r.Dir, "docs", "adr", "README.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(b), "0003-link")
}

// TestGenerateReportsAnUnlistableRecordDirectory checks that a record
// directory that cannot be listed (here, one that is a file) is an error
// that writes no consumer.
func TestGenerateReportsAnUnlistableRecordDirectory(t *testing.T) {
	r := generateFixture(t)
	require.NoError(t, os.RemoveAll(filepath.Join(r.Dir, "docs", "adr")))
	r.Write("docs/adr", "not a directory\n")
	code, out, errOut := runGen(t, r)
	assert.Equal(t, exitError, code)
	assert.Empty(t, out)
	assert.Contains(t, errOut, "docs/adr")
	assert.NoFileExists(t, filepath.Join(r.Dir, ".github", "CODEOWNERS"))
}

// TestGenerateWroteLineIsSafe checks that a path read from .adr-dir
// cannot forge a line of the log through the "wrote" line.
func TestGenerateWroteLineIsSafe(t *testing.T) {
	r := generateFixture(t)
	dir := "d\n\u00a0::error::forged\x1b[2J"
	r.Write(".adr-dir", dir+"\n")
	require.NoError(t, os.MkdirAll(filepath.Join(r.Dir, dir), 0o755))
	code, out, errOut := runGen(t, r)
	require.Equal(t, exitOK, code, errOut)
	assert.Contains(t, out, `d`+"\n"+`\u00a0::error::forged\x1b[2J/README.md`)
	assert.NotContains(t, out, "\x1b")
	for _, line := range strings.Split(out, "\n") {
		assert.False(t, strings.HasPrefix(strings.TrimSpace(line), "::"), "%q", line)
		assert.NotContains(t, line, "\u00a0")
	}
}
