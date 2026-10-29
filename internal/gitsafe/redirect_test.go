// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// moved serves a repository that has moved: the new host serves it
// over dumb HTTP and counts the requests it gets, and the old host
// answers every request with status, a redirect to the same path on
// the new host. It returns the URL of the repository on the old host,
// the git hosts that trust both servers, and the counter.
func moved(t *testing.T, status int) (url string, hosts []GitHost, reached *atomic.Int32) {
	t.Helper()
	src, _ := newRepo(t)
	files := http.FileServer(http.Dir(bareCopy(t, src, "repo.git")))
	reached = new(atomic.Int32)
	newHost, caFile := serveTLS(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		reached.Add(1)
		files.ServeHTTP(w, req)
	}))
	oldHost, _ := serveTLS(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "https://"+newHost+req.URL.RequestURI(), status)
	}))
	hosts = []GitHost{{Host: oldHost, CAFile: caFile}, {Host: newHost, CAFile: caFile}}
	return "https://" + oldHost + "/repo.git", hosts, reached
}

// TestMovedRepository: host mode follows no redirect (05 5.1). A clone
// or a fetch from a repository whose server answers with a redirect to
// another host fails with ErrMoved, which names the URL asked for and
// asks for the new one, and no request reaches the other host. Plain
// git with the same trust follows the redirect (the positive control).
func TestMovedRepository(t *testing.T) {
	for _, status := range []int{
		http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Run("control", func(t *testing.T) {
				url, hosts, reached := moved(t, status)
				dest := filepath.Join(t.TempDir(), "r")
				plainGit(t, filepath.Dir(dest), "-c", "http.sslCAInfo="+hosts[0].CAFile, "clone", "-q", url, dest)
				assert.Positive(t, reached.Load(), "plain git follows the redirect")
				assert.NotEmpty(t, refOf(t, dest, "HEAD"), "and clones from the new host")
			})
			t.Run("clone", func(t *testing.T) {
				url, hosts, reached := moved(t, status)
				r, err := New(Config{Mode: Host, Git: gitPath(t), Home: t.TempDir(), GitHosts: hosts})
				require.NoError(t, err)
				err = r.Clone(t.Context(), url, filepath.Join(t.TempDir(), "r"))
				assertMoved(t, err, url, status)
				assert.Zero(t, reached.Load(), "no request reaches the new host")
			})
			t.Run("fetch", func(t *testing.T) {
				url, hosts, reached := moved(t, status)
				r, err := New(Config{Mode: Host, Git: gitPath(t), Home: t.TempDir(), GitHosts: hosts})
				require.NoError(t, err)
				dir, _ := newRepo(t)
				err = r.Fetch(t.Context(), dir, url, []string{"+refs/heads/*:refs/sandboxes/shop/*"})
				assertMoved(t, err, url, status)
				assert.Zero(t, reached.Load(), "no request reaches the new host")
			})
		})
	}
}

// assertMoved holds err to the error of a moved repository at url
// whose server answered status.
func assertMoved(t *testing.T, err error, url string, status int) {
	t.Helper()
	require.ErrorIs(t, err, ErrMoved)
	assert.Contains(t, err.Error(), url)
	assert.Contains(t, err.Error(), "set its new URL")
	assert.Equal(t, 1, strings.Count(err.Error(), "HTTP "+strconv.Itoa(status)+","), err.Error())
}

// TestMovedErrorReadsGitsLineOnly: only the line git writes last for a
// redirect of the URL it fetched makes ErrMoved; a line a server wrote
// or made git echo, a redirect of another URL, another status, another
// failure or a standard error cut at its limit leaves the error as it
// was.
func TestMovedErrorReadsGitsLineOnly(t *testing.T) {
	const url = "https://git.example.com/o/r.git"
	const redirect = "fatal: unable to access '" + url + "/': The requested URL returned error: 301\n"
	run := errors.New("git exited 128")
	tests := []struct {
		name, stderr, resolved string
		truncated              bool
		moved                  bool
	}{
		{name: "redirect", stderr: "Cloning into 'r'...\n" + redirect, resolved: url, moved: true},
		{name: "rewritten", stderr: "fatal: unable to access 'https://mirror.example.com/r.git/': The requested URL returned error: 308\n", resolved: "https://mirror.example.com/r.git", moved: true},
		{name: "an apostrophe", stderr: "fatal: unable to access 'https://git.example.com/it's.git/': The requested URL returned error: 302\n", resolved: "https://git.example.com/it's.git", moved: true},
		{name: "from the server", stderr: "remote: " + redirect, resolved: url},
		{name: "echoed from the server", stderr: "fatal: git fetch-pack: expected ACK/NAK, got 'x\n" + redirect + "'\nfatal: the remote end hung up unexpectedly\n", resolved: url},
		{name: "cut at the limit", stderr: "fatal: git fetch-pack: expected ACK/NAK, got 'x\n" + redirect, resolved: url, truncated: true},
		{name: "another URL", stderr: "fatal: unable to access 'https://elsewhere.example.com/r.git/': The requested URL returned error: 301\n", resolved: url},
		{name: "a longer URL", stderr: "fatal: unable to access '" + url + "x/': The requested URL returned error: 301\n", resolved: url},
		{name: "not a redirect", stderr: "fatal: unable to access '" + url + "/': The requested URL returned error: 404\n", resolved: url},
		{name: "not modified", stderr: "fatal: unable to access '" + url + "/': The requested URL returned error: 304\n", resolved: url},
		{name: "another failure", stderr: "fatal: repository '" + url + "/' not found\n", resolved: url},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := movedError(withRedirect(run, tt.stderr, tt.truncated), url, tt.resolved)
			require.ErrorIs(t, err, run)
			if !tt.moved {
				assert.NotErrorIs(t, err, ErrMoved)
				return
			}
			require.ErrorIs(t, err, ErrMoved)
			assert.Contains(t, err.Error(), url)
			assert.Contains(t, err.Error(), tt.resolved)
		})
	}
	t.Run("no error", func(t *testing.T) {
		assert.NoError(t, movedError(nil, url, url))
	})
}
