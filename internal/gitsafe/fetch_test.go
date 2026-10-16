// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFetchArgv: a fetch carries the four flags of 05 5.1 and an
// explicit URL with refspecs, never a remote name, after the
// resolution of the URL.
func TestFetchArgv(t *testing.T) {
	r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: "/h"})
	require.NoError(t, err)
	var rec recorder
	rec.install(r)
	const url = "https://github.com/o/r.git"
	specs := []string{"+refs/heads/*:refs/romeu/origin/r/*", "refs/heads/main:refs/romeu/origin/r/pinned"}
	require.NoError(t, r.Fetch(t.Context(), stubRepo, url, specs))
	require.Len(t, rec.commands, 2)

	resolve := rec.commands[0]
	assert.Equal(t, []string{"ls-remote", "--get-url", url}, resolve.Argv[1+len(hostFlags):])
	assert.Equal(t, stubRepo, resolve.Dir)

	fetch := rec.commands[1]
	assert.Equal(t, stubRepo, fetch.Dir)
	assert.Equal(t,
		append([]string{"fetch", "--no-tags", "--no-prune", "--no-recurse-submodules", "--no-write-fetch-head", url}, specs...),
		fetch.Argv[1+len(hostFlags):])
	assert.NotContains(t, fetch.Argv, "origin")
}

// TestFetch fetches over the loopback daemon in each mode into the
// allowlisted namespace, and a fetch that brings nothing writes
// nothing (the idempotency rule of 04 4.1).
func TestFetch(t *testing.T) {
	src, head := newRepo(t)
	url := serveGit(t, bareCopy(t, src, "repo.git")) + "repo.git"
	for _, tt := range []struct {
		mode Mode
		dst  string
	}{{Host, "refs/sandboxes/shop/"}, {Sandbox, "refs/remotes/origin/"}} {
		t.Run(tt.mode.String(), func(t *testing.T) {
			dir, _ := newRepo(t)
			r := newRunner(t, tt.mode)
			before := plainGit(t, dir, "for-each-ref", "refs/heads", "refs/tags", "refs/remotes")
			beforeHead := plainGit(t, dir, "rev-parse", "HEAD")
			require.NoError(t, r.Fetch(t.Context(), dir, url, []string{"+refs/heads/*:" + tt.dst + "*"}))
			assert.Equal(t, head, refOf(t, dir, tt.dst+"main"))
			assert.NoFileExists(t, filepath.Join(dir, ".git", "FETCH_HEAD"))
			if tt.mode == Host {
				assert.Equal(t, before, plainGit(t, dir, "for-each-ref", "refs/heads", "refs/tags", "refs/remotes"))
			}
			assert.Equal(t, beforeHead, plainGit(t, dir, "rev-parse", "HEAD"))

			refs := plainGit(t, dir, "for-each-ref")
			require.NoError(t, r.Fetch(t.Context(), dir, url, []string{"+refs/heads/*:" + tt.dst + "*"}))
			assert.Equal(t, refs, plainGit(t, dir, "for-each-ref"), "a second fetch changes nothing")
			assert.NoFileExists(t, filepath.Join(dir, ".git", "FETCH_HEAD"))
		})
	}
}

// TestFetchCAFile fetches over HTTPS from a server whose certificate
// only its caFile trusts: the caFile of that host lets the fetch
// through, and with no caFile, or with the caFile set for another host
// only, the fetch fails.
func TestFetchCAFile(t *testing.T) {
	src, head := newRepo(t)
	host, caFile := serveHTTPS(t, bareCopy(t, src, "repo.git"))
	url := "https://" + host + "/repo.git"
	specs := []string{"+refs/heads/*:refs/romeu/origin/app/*"}
	tests := []struct {
		name  string
		hosts []GitHost
		ok    bool
	}{
		{"its own host", []GitHost{{Host: host, CAFile: caFile}}, true},
		{"no ca file", []GitHost{{Host: host}}, false},
		{"another host only", []GitHost{{Host: "github.com", CAFile: caFile}, {Host: host}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, _ := newRepo(t)
			r, err := New(Config{Mode: Host, Git: gitPath(t), Home: t.TempDir(), GitHosts: tt.hosts})
			require.NoError(t, err)
			err = r.Fetch(t.Context(), dir, url, specs)
			if !tt.ok {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), "certificate")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, head, refOf(t, dir, "refs/romeu/origin/app/main"))
		})
	}
}

// TestFastForward moves the checked-out branch of a sandbox clone
// forward, and refuses a move that is not a fast-forward, and the host.
func TestFastForward(t *testing.T) {
	src, _ := newRepo(t)
	dir := t.TempDir()
	plainGit(t, dir, "clone", "-q", src, "clone")
	dir = filepath.Join(dir, "clone")
	next := commit(t, src, "two")
	url := serveGit(t, bareCopy(t, src, "repo.git")) + "repo.git"

	r := newRunner(t, Sandbox)
	require.NoError(t, r.Fetch(t.Context(), dir, url, []string{"+refs/heads/*:refs/remotes/origin/*"}))
	require.NoError(t, r.FastForward(t.Context(), dir, next))
	assert.Equal(t, next, plainGit(t, dir, "rev-parse", "HEAD"))
	assert.Equal(t, next, refOf(t, dir, "refs/heads/main"))

	local := commit(t, dir, "local")
	other := commit(t, src, "three")
	plainGit(t, dir, "fetch", "-q", src, "main")
	require.Error(t, r.FastForward(t.Context(), dir, other), "diverged")
	assert.Equal(t, local, plainGit(t, dir, "rev-parse", "HEAD"))

	require.ErrorIs(t, r.FastForward(t.Context(), dir, "main"), ErrRefused)
	require.ErrorIs(t, newRunner(t, Host).FastForward(t.Context(), dir, other), ErrRefused, "the host never fast-forwards a branch (I5)")
}
