// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tipOrigin serves a repository over git:// whose main holds a lock
// file and two commits, a branch bare without it, a branch link whose
// lock is a symbolic link, and a branch big whose lock is over 64
// bytes, and returns its URL.
func tipOrigin(t *testing.T) string {
	t.Helper()
	src, _ := newRepo(t)
	write := func(name, body string) {
		require.NoError(t, os.WriteFile(filepath.Join(src, name), []byte(body), 0o600))
	}
	write("mise.lock", "old\n")
	plainGit(t, src, "add", "mise.lock")
	commit(t, src, "old lock")
	write("mise.lock", "lock\n")
	plainGit(t, src, "add", "mise.lock")
	commit(t, src, "lock")
	plainGit(t, src, "checkout", "-q", "-b", "bare")
	plainGit(t, src, "rm", "-q", "mise.lock")
	commit(t, src, "no lock")
	plainGit(t, src, "checkout", "-q", "-b", "link", "main")
	plainGit(t, src, "rm", "-q", "mise.lock")
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(src, "mise.lock")))
	plainGit(t, src, "add", "mise.lock")
	commit(t, src, "a link")
	plainGit(t, src, "checkout", "-q", "-b", "big", "main")
	write("mise.lock", strings.Repeat("x", 65))
	plainGit(t, src, "add", "mise.lock")
	commit(t, src, "big")
	plainGit(t, src, "checkout", "-q", "main")
	return serveGit(t, bareCopy(t, src, "repo.git")) + "repo.git"
}

// TestTipFile reads the file at the tip of the default branch and of a
// named branch, from that commit alone: a shallow repository of one
// commit, with no checkout and no tag; a branch without the file gives
// none and no error.
func TestTipFile(t *testing.T) {
	url := tipOrigin(t)
	r := newRunner(t, Sandbox)
	for _, ref := range []string{"", "main"} {
		work := t.TempDir()
		data, found, err := r.TipFile(t.Context(), work, url, ref, "mise.lock", 64)
		require.NoError(t, err, ref)
		assert.True(t, found, ref)
		assert.Equal(t, "lock\n", string(data), ref)
		repo := filepath.Join(work, tipRepo)
		assert.Equal(t, "1", plainGit(t, repo, "rev-list", "--count", "refs/remotes/origin/main"), "one commit, depth 1")
		_, err = os.Stat(filepath.Join(repo, "shallow"))
		require.NoError(t, err, "a shallow repository")
		assert.Empty(t, plainGit(t, repo, "for-each-ref", "refs/tags", "refs/heads"))
	}
	_, found, err := r.TipFile(t.Context(), t.TempDir(), url, "bare", "mise.lock", 64)
	require.NoError(t, err)
	assert.False(t, found)
}

// TestTipFileRefusesTheFile: a lock that is a symbolic link, or one over
// the limit, is refused as a file (ErrFileRefused), not as a failure of
// the fetch.
func TestTipFileRefusesTheFile(t *testing.T) {
	url := tipOrigin(t)
	r := newRunner(t, Sandbox)
	for _, ref := range []string{"link", "big"} {
		_, _, err := r.TipFile(t.Context(), t.TempDir(), url, ref, "mise.lock", 64)
		require.ErrorIs(t, err, ErrFileRefused, ref)
		assert.ErrorIs(t, err, ErrRefused, ref)
	}
}

// TestTipFileMissingRef: a branch the origin lacks fails the fetch, and
// an empty origin names no default branch to read; neither is a
// refusal of content.
func TestTipFileMissingRef(t *testing.T) {
	url := tipOrigin(t)
	r := newRunner(t, Sandbox)
	_, _, err := r.TipFile(t.Context(), t.TempDir(), url, "gone", "mise.lock", 64)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrRefused)

	base := t.TempDir()
	plainGit(t, base, "init", "-q", "--bare", "empty.git")
	empty := serveGit(t, base) + "empty.git"
	_, _, err = r.TipFile(t.Context(), t.TempDir(), empty, "", "mise.lock", 64)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrRefused)
}

// TestTipFileRefuses: host mode, a URL CheckURL refuses, a branch name
// gitsafe refuses and a work directory that is not absolute are refused
// before git runs.
func TestTipFileRefuses(t *testing.T) {
	sandbox := newRunner(t, Sandbox)
	var rec recorder
	rec.install(sandbox)
	for name, call := range map[string]func() error{
		"host": func() error {
			_, _, err := newRunner(t, Host).TipFile(t.Context(), t.TempDir(), "https://example.com/o/r", "main", "mise.lock", 64)
			return err
		},
		"url": func() error {
			_, _, err := sandbox.TipFile(t.Context(), t.TempDir(), "file:///tmp/r", "main", "mise.lock", 64)
			return err
		},
		"work": func() error {
			_, _, err := sandbox.TipFile(t.Context(), "rel", "https://example.com/o/r", "main", "mise.lock", 64)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.ErrorIs(t, call(), ErrRefused)
		})
	}
	assert.Empty(t, rec.commands, "no git ran")

	_, _, err := newRunner(t, Sandbox).TipFile(t.Context(), t.TempDir(), tipOrigin(t), "a..b", "mise.lock", 64)
	assert.ErrorIs(t, err, ErrRefused, "a branch name gitsafe refuses")
}

// TestTipFileAdvertisedHEAD: the default branch is read from the
// origin's HEAD, a symbolic ref, so a HEAD on another branch is read.
func TestTipFileAdvertisedHEAD(t *testing.T) {
	src, _ := newRepo(t)
	plainGit(t, src, "checkout", "-q", "-b", "trunk")
	require.NoError(t, os.WriteFile(filepath.Join(src, "mise.lock"), []byte("trunk\n"), 0o600))
	plainGit(t, src, "add", "mise.lock")
	commit(t, src, "trunk")
	base := bareCopy(t, src, "repo.git")
	plainGit(t, filepath.Join(base, "repo.git"), "symbolic-ref", "HEAD", "refs/heads/trunk")
	url := serveGit(t, base) + "repo.git"

	data, found, err := newRunner(t, Sandbox).TipFile(t.Context(), t.TempDir(), url, "", "mise.lock", 64)

	require.NoError(t, err)
	assert.True(t, found)
	assert.True(t, bytes.Equal([]byte("trunk\n"), data))
}
