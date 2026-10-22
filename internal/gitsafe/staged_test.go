// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStagedRealGit lists the staged paths among the names given: a
// path added, changed and deleted in the index each count, an unstaged
// change and an untracked file do not, and the answer is sorted.
func TestStagedRealGit(t *testing.T) {
	dir, _ := newRepo(t)
	for _, mode := range []Mode{Host, Sandbox} {
		t.Run(mode.String(), func(t *testing.T) {
			got, err := newRunner(t, mode).Staged(t.Context(), dir, "", "mise.toml", "mise.lock")
			require.NoError(t, err)
			assert.Empty(t, got, "a clean tree stages nothing")
		})
	}
	r := newRunner(t, Sandbox)
	writeFile(t, dir, "mise.toml", "a")
	writeFile(t, dir, "mise.lock", "a")
	writeFile(t, dir, "other", "a")
	plainGit(t, dir, "add", "mise.toml", "other")
	got, err := r.Staged(t.Context(), dir, "", "mise.toml", "mise.lock")
	require.NoError(t, err)
	assert.Equal(t, []string{"mise.toml"}, got, "mise.lock is untracked, not staged")

	plainGit(t, dir, "add", "mise.lock")
	plainGit(t, dir, "commit", "-q", "-m", "files")
	writeFile(t, dir, "mise.toml", "b")
	got, err = r.Staged(t.Context(), dir, "", "mise.toml", "mise.lock")
	require.NoError(t, err)
	assert.Empty(t, got, "an unstaged change is not staged")
	plainGit(t, dir, "add", "mise.toml")
	plainGit(t, dir, "rm", "-q", "--cached", "mise.lock")
	got, err = r.Staged(t.Context(), dir, "", "mise.toml", "mise.lock")
	require.NoError(t, err)
	assert.Equal(t, []string{"mise.lock", "mise.toml"}, got, "a change and a deletion, sorted")
	got, err = r.Staged(t.Context(), dir, "", "mise.lock", "mise.toml")
	require.NoError(t, err)
	assert.Equal(t, []string{"mise.lock", "mise.toml"}, got, "sorted whatever the order of the names")
}

// TestStagedRefuses: a name that is no literal path of the work tree is
// refused before git starts, each in its own way (ErrRefused).
func TestStagedRefuses(t *testing.T) {
	dir, _ := newRepo(t)
	r := newRunner(t, Sandbox)
	for _, name := range []string{"mise.*", "m?se", "[m]ise", `m\ise`, ":(glob)mise.toml", ":mise.toml", "/etc/passwd", "", "../x", "a/../b", "./mise.toml", "a//b", "a/./b", "mise\ntoml", "mise\x00toml"} {
		got, err := r.Staged(t.Context(), dir, "", name)
		require.ErrorIs(t, err, ErrRefused, "%q", name)
		assert.Empty(t, got)
		_, _, err = r.IndexFile(t.Context(), dir, "", name, 10)
		require.ErrorIs(t, err, ErrRefused, "%q", name)
	}
	_, err := r.Staged(t.Context(), dir, "")
	require.ErrorIs(t, err, ErrRefused, "no name")
	_, err = r.Staged(t.Context(), dir, "relative/index", "mise.toml")
	require.ErrorIs(t, err, ErrRefused, "a relative index file")
	_, err = r.Staged(t.Context(), dir, dir+"/../x", "mise.toml")
	require.ErrorIs(t, err, ErrRefused, "an unclean index file")
	_, err = r.Staged(t.Context(), dir, dir+"/.git/index\n", "mise.toml")
	require.ErrorIs(t, err, ErrRefused, "a control character in the index file")
}

// TestStagedRunsNoExternalProgram: a diff.external and a textconv of the
// repo's config or attributes run nothing, in Staged and in the read
// of a blob (the flags of the call, 05 5.1).
func TestStagedRunsNoExternalProgram(t *testing.T) {
	dir, _ := newRepo(t)
	marker := filepath.Join(t.TempDir(), "marker")
	script := filepath.Join(t.TempDir(), "run")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\necho ran >> '"+marker+"'\ncat \"$1\"\n"), 0o700))
	plainGit(t, dir, "config", "diff.external", script)
	plainGit(t, dir, "config", "diff.x.textconv", script)
	writeFile(t, dir, ".gitattributes", "mise.toml diff=x\n")
	writeFile(t, dir, "mise.toml", "a")
	plainGit(t, dir, "add", "mise.toml", ".gitattributes")
	plainGit(t, dir, "commit", "-q", "-m", "files")
	writeFile(t, dir, "mise.toml", "b")
	plainGit(t, dir, "add", "mise.toml")
	r := newRunner(t, Sandbox)
	got, err := r.Staged(t.Context(), dir, "", "mise.toml")
	require.NoError(t, err)
	assert.Equal(t, []string{"mise.toml"}, got)
	data, found, err := r.IndexFile(t.Context(), dir, "", "mise.toml", 100)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "b", string(data))
	assert.NoFileExists(t, marker)
	plainGit(t, dir, "diff", "--cached")
	assert.FileExists(t, marker, "the plant is live: plain git runs it")
}

// tempIndex copies the index of dir to a file of the git dir, as git
// commit does for a commit that stages for itself, and returns it.
func tempIndex(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, ".git", "index"))
	require.NoError(t, err)
	p := filepath.Join(dir, ".git", "next-index-1.lock")
	require.NoError(t, os.WriteFile(p, b, 0o600))
	return p
}

// TestStagedOfAnotherIndex reads the index a commit that stages for
// itself gives its hook: git commit -a updates every tracked file in a
// temporary index, and the repository's own index does not hold it.
func TestStagedOfAnotherIndex(t *testing.T) {
	dir, _ := newRepo(t)
	writeFile(t, dir, "mise.toml", "a")
	plainGit(t, dir, "add", "mise.toml")
	plainGit(t, dir, "commit", "-q", "-m", "files")
	writeFile(t, dir, "mise.toml", "b")
	idx := tempIndex(t, dir)
	_, err := plainGitErr(t, dir, []string{"GIT_INDEX_FILE=" + idx}, "add", "-u")
	require.NoError(t, err)

	r := newRunner(t, Sandbox)
	got, err := r.Staged(t.Context(), dir, "", "mise.toml")
	require.NoError(t, err)
	assert.Empty(t, got, "the repository's own index stages nothing")
	got, err = r.Staged(t.Context(), dir, idx, "mise.toml")
	require.NoError(t, err)
	assert.Equal(t, []string{"mise.toml"}, got)
	data, found, err := r.IndexFile(t.Context(), dir, idx, "mise.toml", 100)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "b", string(data), "the temporary index holds the new content")
	data, _, err = r.IndexFile(t.Context(), dir, "", "mise.toml", 100)
	require.NoError(t, err)
	assert.Equal(t, "a", string(data))
}

// TestIndexFile reads a blob of the index: not the work tree's content,
// a missing path is not found, and the read is bounded.
func TestIndexFile(t *testing.T) {
	dir, _ := newRepo(t)
	r := newRunner(t, Sandbox)
	writeFile(t, dir, "mise.toml", "staged")
	plainGit(t, dir, "add", "mise.toml")
	writeFile(t, dir, "mise.toml", "work tree")
	data, found, err := r.IndexFile(t.Context(), dir, "", "mise.toml", 100)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "staged", string(data))

	_, found, err = r.IndexFile(t.Context(), dir, "", "mise.lock", 100)
	require.NoError(t, err)
	assert.False(t, found)

	data, found, err = r.IndexFile(t.Context(), dir, "", "mise.toml", len("staged"))
	require.NoError(t, err, "a blob of the limit is read")
	assert.True(t, found)
	assert.Equal(t, "staged", string(data))
	_, _, err = r.IndexFile(t.Context(), dir, "", "mise.toml", len("staged")-1)
	require.ErrorIs(t, err, ErrRefused, "a blob over the limit")
}

// TestIndexFileRefusesWhatIsNoRegularFile: a link and a submodule entry
// are refused, and so is an unmerged one.
func TestIndexFileRefusesWhatIsNoRegularFile(t *testing.T) {
	dir, _ := newRepo(t)
	r := newRunner(t, Sandbox)
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(dir, "mise.lock")))
	plainGit(t, dir, "add", "mise.lock")
	_, _, err := r.IndexFile(t.Context(), dir, "", "mise.lock", 100)
	require.ErrorIs(t, err, ErrRefused)
	assert.Contains(t, err.Error(), "not a regular file")

	writeFile(t, dir, "mise.toml", "base")
	plainGit(t, dir, "add", "mise.toml")
	plainGit(t, dir, "commit", "-q", "-m", "base")
	plainGit(t, dir, "checkout", "-q", "-b", "side")
	writeFile(t, dir, "mise.toml", "side")
	plainGit(t, dir, "commit", "-q", "-am", "side")
	plainGit(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "mise.toml", "main")
	plainGit(t, dir, "commit", "-q", "-am", "main")
	_, err = plainGitErr(t, dir, nil, "merge", "side")
	require.Error(t, err, "the merge conflicts")
	_, _, err = r.IndexFile(t.Context(), dir, "", "mise.toml", 100)
	require.ErrorIs(t, err, ErrRefused)
	assert.Contains(t, err.Error(), "unmerged")
}

// TestStagedUnborn reads the index of a repository with no commit yet.
func TestStagedUnborn(t *testing.T) {
	dir := t.TempDir()
	plainGit(t, dir, "init", "-q")
	writeFile(t, dir, "mise.toml", "a")
	plainGit(t, dir, "add", "mise.toml")
	got, err := newRunner(t, Sandbox).Staged(t.Context(), dir, "", "mise.toml")
	require.NoError(t, err)
	assert.Equal(t, []string{"mise.toml"}, got)
}
