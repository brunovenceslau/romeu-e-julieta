// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package drift

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
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/askfirst"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// generateFixture is a repository with the sources of generate: the
// ask-first list, .adr-dir and two records.
func generateFixture(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	r.Write(askfirst.Path, citest.ValidAskFirst)
	r.Write(".adr-dir", "docs/adr\n")
	r.Write("docs/adr/0001-first.md", "# 1. First\n\n## Status\n\nAccepted\n")
	r.Write("docs/adr/0002-second.md", "# 2. Second\n\n## Status\n\nProposed\n")
	return r
}

func runGen(t *testing.T, r *gittest.Repo, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	e := check.Env{Dir: r.Dir, GitEnv: r.Env, Stdout: &out, Stderr: &errOut}
	return citest.Exit(context.Background(), e, Generate, args...), out.String(), errOut.String()
}

// TestGenerateWritesTheConsumers is the acceptance case: from the
// ask-first list and the titles and statuses of the records, generate
// writes CODEOWNERS and the index.
func TestGenerateWritesTheConsumers(t *testing.T) {
	r := generateFixture(t)
	code, out, errOut := runGen(t, r)
	require.Equal(t, check.ExitOK, code, errOut)
	assert.Equal(t, "wrote .github/CODEOWNERS\nwrote docs/adr/README.md\n", out)
	read := func(p string) string {
		b, err := os.ReadFile(filepath.Join(r.Dir, filepath.FromSlash(p)))
		require.NoError(t, err)
		return string(b)
	}
	assert.Contains(t, read(".github/CODEOWNERS"), "/.github/ask-first.yaml @someone\n")
	index := read("docs/adr/README.md")
	assert.Contains(t, index, "| 1 | [First](0001-first.md) | Accepted |\n")
	assert.Contains(t, index, "| 2 | [Second](0002-second.md) | Proposed |\n")
}

// TestGenerateIsIdempotent checks that a second run writes nothing and
// leaves the modification times alone.
func TestGenerateIsIdempotent(t *testing.T) {
	r := generateFixture(t)
	code, _, errOut := runGen(t, r)
	require.Equal(t, check.ExitOK, code, errOut)
	path := filepath.Join(r.Dir, ".github", "CODEOWNERS")
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(path, old, old))
	code, out, errOut := runGen(t, r)
	require.Equal(t, check.ExitOK, code, errOut)
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
	require.Equal(t, check.ExitOK, code)
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
		{"no ask-first list", func(r *gittest.Repo) { require.NoError(t, os.Remove(filepath.Join(r.Dir, askfirst.Path))) }, nil, "ask-first.yaml"},
		{"an invalid list", func(r *gittest.Repo) { r.Write(askfirst.Path, "version: 3\n") }, nil, "version is 3"},
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
			require.NoError(t, os.WriteFile(outside, []byte(citest.ValidAskFirst), 0o644))
			require.NoError(t, os.Remove(filepath.Join(r.Dir, askfirst.Path)))
			require.NoError(t, os.Symlink(outside, filepath.Join(r.Dir, askfirst.Path)))
		}, nil, "ask-first.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := generateFixture(t)
			if tt.setup != nil {
				tt.setup(r)
			}
			code, out, errOut := runGen(t, r, tt.args...)
			assert.Equal(t, check.ExitError, code)
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
	require.Equal(t, check.ExitOK, code, errOut)
	b, err := os.ReadFile(filepath.Join(r.Dir, "docs", "adr", "README.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(b), "0009")
	assert.Contains(t, string(b), "0002-second.md")
}

// TestGenerateWritesNothingOutsideTheTree is the case of a generated
// file committed as a link to a path outside the repository: generate
// replaces the link, and the file it pointed to is untouched.
func TestGenerateWritesNothingOutsideTheTree(t *testing.T) {
	r := generateFixture(t)
	victim := filepath.Join(t.TempDir(), "victim")
	require.NoError(t, os.WriteFile(victim, []byte("mine\n"), 0o644))
	page := filepath.Join(r.Dir, "docs", "adr", "README.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(page), 0o755))
	require.NoError(t, os.Symlink(victim, page))
	code, _, errOut := runGen(t, r)
	require.Equal(t, check.ExitOK, code, errOut)
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
	require.Equal(t, check.ExitOK, code, errOut)
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
	assert.Equal(t, check.ExitError, code)
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
	require.Equal(t, check.ExitOK, code, errOut)
	assert.Contains(t, out, `d`+"\n"+`\u00a0::error::forged\x1b[2J/README.md`)
	assert.NotContains(t, out, "\x1b")
	for _, line := range strings.Split(out, "\n") {
		assert.False(t, strings.HasPrefix(strings.TrimSpace(line), "::"), "%q", line)
		assert.NotContains(t, line, "\u00a0")
	}
}

// TestGeneratedOfThisRepository runs the generators on this repository
// and checks that each committed output is what they produce, CODEOWNERS
// and the index of the decision records among them, so a hand edit of
// either fails here before it fails "generated".
func TestGeneratedOfThisRepository(t *testing.T) {
	dir := citest.ModuleRoot(t)
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })
	outputs, err := generateOutputs(root)
	require.NoError(t, err)
	var paths []string
	for _, out := range outputs {
		paths = append(paths, out.path)
	}
	require.Subset(t, paths, []string{".github/CODEOWNERS", "docs/adr/README.md"})
	for _, out := range outputs {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(out.path)))
		require.NoError(t, err, out.path)
		assert.Equal(t, string(out.content), string(got), "%s is current", out.path)
	}
}
