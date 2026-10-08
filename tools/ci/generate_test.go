// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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

// TestWriteIfChangedReportsAnUnwritableTarget checks the one error path
// of the writer that the fixtures above cannot reach.
func TestWriteIfChangedReportsAnUnwritableTarget(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "d"), 0o755))
	_, err := writeIfChanged(filepath.Join(dir, "d"), []byte("x"))
	require.Error(t, err, "a directory is not a file to read")
	file := filepath.Join(dir, "f")
	require.NoError(t, os.WriteFile(file, nil, 0o644))
	_, err = writeIfChanged(filepath.Join(file, "child"), []byte("x"))
	require.Error(t, err, "a file is not a directory to create in")
}
