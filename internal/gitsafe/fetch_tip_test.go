// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
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

// TestTipFileNoDefaultBranch: an origin whose HEAD is detached, or names
// a ref outside refs/heads/, names no default branch: ErrNoDefaultBranch,
// a state of the origin and not a refusal (no ErrRefused).
func TestTipFileNoDefaultBranch(t *testing.T) {
	src, _ := newRepo(t)
	plainGit(t, src, "tag", "v1")
	base := bareCopy(t, src, "repo.git")
	repo := filepath.Join(base, "repo.git")
	url := serveGit(t, base) + "repo.git"
	r := newRunner(t, Sandbox)
	for name, point := range map[string][]string{
		"detached": {"update-ref", "--no-deref", "HEAD", "refs/heads/main"},
		"a tag":    {"symbolic-ref", "HEAD", "refs/tags/v1"},
	} {
		t.Run(name, func(t *testing.T) {
			plainGit(t, repo, point...)
			_, _, err := r.TipFile(t.Context(), t.TempDir(), url, "", "mise.lock", 64)
			require.ErrorIs(t, err, ErrNoDefaultBranch)
			assert.NotErrorIs(t, err, ErrRefused)
		})
	}
}

// TestTipFileFlags: the ls-remote and the fetch of TipFile run with its
// own hardening, as host mode's every call does (05 5.1): no redirect
// followed, no automatic gc or maintenance, every object checked; the
// other calls of sandbox mode do not get them.
func TestTipFileFlags(t *testing.T) {
	r := newRunner(t, Sandbox)
	var rec recorder
	rec.install(r)
	_, _, _ = r.TipFile(t.Context(), t.TempDir(), "https://example.com/o/r", "", "mise.lock", 64)
	_, _, _ = r.TipFile(t.Context(), t.TempDir(), "https://example.com/o/r", "main", "mise.lock", 64)
	want := []string{"-c", "http.followRedirects=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false", "-c", "transfer.fsckObjects=true"}
	seen := map[string]bool{}
	for _, c := range rec.commands {
		verb := ""
		for i, a := range c.Argv[1:] {
			if a != "-c" && (i == 0 || c.Argv[i] != "-c") {
				verb = a
				break
			}
		}
		has := strings.Contains(strings.Join(c.Argv, "\x00"), strings.Join(want, "\x00"))
		switch {
		case verb == "fetch" || verb == "ls-remote" && slices.Contains(c.Argv, "--symref"):
			assert.True(t, has, "%s runs with the flags of TipFile: %q", verb, c.Argv)
			seen[verb] = true
		case verb == "init":
			assert.False(t, has, "init runs with the flags of the mode only")
		}
	}
	assert.True(t, seen["fetch"], "a fetch ran")
	assert.True(t, seen["ls-remote"], "an ls-remote --symref ran")
}

// TestTipFileFollowsNoRedirect: an origin whose server answers with a
// redirect fails with ErrMoved, and no request reaches the other host,
// for the read of the default branch and for the fetch. The sandbox's
// own global config trusts the test CA (http.sslCAInfo), as a sandbox's
// git config is its own in sandbox mode. The network pass-through of
// sandbox mode is emptied first, so a proxy of the shell that runs the
// test does not carry the request to 127.0.0.1.
func TestTipFileFollowsNoRedirect(t *testing.T) {
	for _, k := range NetworkEnv() {
		t.Setenv(k, "")
	}
	for _, ref := range []string{"", "main"} {
		url, hosts, reached := moved(t, http.StatusMovedPermanently)
		home := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[http]\n\tsslCAInfo = "+hosts[0].CAFile+"\n"), 0o600))
		r, err := New(Config{Mode: Sandbox, Git: gitPath(t), Home: home})
		require.NoError(t, err)
		_, _, err = r.TipFile(t.Context(), t.TempDir(), url, ref, "mise.lock", 64)
		require.ErrorIs(t, err, ErrMoved, "ref %q: %v", ref, err)
		assert.Zero(t, reached.Load(), "no request reaches the new host")
	}
}

// remoteAnswering returns a sandbox runner whose git ls-remote prints
// out and exits 0, and whose other calls start nothing.
func remoteAnswering(t *testing.T, out string) *Runner {
	t.Helper()
	r := newRunner(t, Sandbox)
	r.start = func(_ context.Context, c procgroup.Command) error {
		if slices.Contains(c.Argv, "ls-remote") && c.Stdout != nil {
			_, err := io.WriteString(c.Stdout, out)
			return err
		}
		return nil
	}
	return r
}

// TestRemoteDefaultRefusedName: a HEAD that names a branch gitsafe does
// not accept (not valid UTF-8, holding ..) names no default branch to
// read: ErrNoDefaultBranch, not a refusal of the caller's input.
func TestRemoteDefaultRefusedName(t *testing.T) {
	for _, target := range []string{"refs/heads/\xff", "refs/heads/a..b", "refs/heads/x.lock"} {
		r := remoteAnswering(t, "ref: "+target+"\tHEAD\n"+strings.Repeat("a", 40)+"\tHEAD\n")
		_, err := r.remoteDefault(t.Context(), stubRepo, "https://example.com/o/r", "https://example.com/o/r")
		require.ErrorIs(t, err, ErrNoDefaultBranch, target)
		assert.NotErrorIs(t, err, ErrRefused, target)
	}
	r := remoteAnswering(t, "ref: refs/heads/trunk\tHEAD\n"+strings.Repeat("a", 40)+"\tHEAD\n")
	branch, err := r.remoteDefault(t.Context(), stubRepo, "https://example.com/o/r", "https://example.com/o/r")
	require.NoError(t, err, "the control")
	assert.Equal(t, "trunk", branch)
}

// TestRemoteDefaultOversize: an advertisement over outputLimit is read
// no further and names no default branch: ErrNoDefaultBranch, even when
// a usable HEAD line comes first.
func TestRemoteDefaultOversize(t *testing.T) {
	r := remoteAnswering(t, "ref: refs/heads/main\tHEAD\n"+strings.Repeat("x", outputLimit+1))

	_, err := r.remoteDefault(t.Context(), stubRepo, "https://example.com/o/r", "https://example.com/o/r")

	require.ErrorIs(t, err, ErrNoDefaultBranch)
	assert.NotErrorIs(t, err, ErrRefused)
	assert.Contains(t, err.Error(), "advertises more than")
}
