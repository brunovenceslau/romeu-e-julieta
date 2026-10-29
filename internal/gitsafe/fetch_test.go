// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
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

// TestFetchRefusesRefspecPatternIntoOrigin: a pattern into
// refs/romeu/origin/ whose * is not a whole last component could build
// <dir>/HEAD from a ref not named HEAD, so it is refused before git
// runs.
//
//romeu:invariant I5 test
func TestFetchRefusesRefspecPatternIntoOrigin(t *testing.T) {
	for _, spec := range []string{
		"+refs/heads/*:refs/romeu/origin/x/H*",
		"+refs/heads/*:refs/romeu/origin/x/HE*",
		"+refs/heads/*:refs/romeu/origin/x/*D",
		"+refs/heads/*:refs/romeu/origin/x/*/y",
	} {
		t.Run(spec, func(t *testing.T) {
			r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: "/h"})
			require.NoError(t, err)
			var rec recorder
			rec.install(r)
			err = r.Fetch(t.Context(), stubRepo, "https://github.com/o/r.git", []string{spec})
			require.ErrorIs(t, err, ErrRefused)
			assert.Contains(t, err.Error(), spec)
			assert.Empty(t, rec.commands, "refused before git runs")
		})
	}
}

// advertiseHEAD makes the recorded runner r answer ls-remote --symref
// with the advertisement out, after the recorder records the call.
func advertiseHEAD(r *Runner, out string) {
	recorded := r.start
	r.start = func(ctx context.Context, c procgroup.Command) error {
		if err := recorded(ctx, c); err != nil {
			return err
		}
		if slices.Contains(c.Argv, "--symref") && c.Stdout != nil {
			_, err := io.WriteString(c.Stdout, out)
			return err
		}
		return nil
	}
}

// liveHEAD is the advertisement of a remote whose HEAD names a branch.
var liveHEAD = "ref: refs/heads/main\tHEAD\n" + strings.Repeat("a", 40) + "\tHEAD\n"

// TestFetchArgv: a fetch carries the four flags of 05 5.1 and an
// explicit URL with refspecs, never a remote name, after the
// resolution of the URL; a fetch into refs/romeu/origin/ carries,
// once, the negative refspecs of every name git reads HEAD as but HEAD
// itself, and a fetch of the remote's HEAD first asks the remote for
// its HEAD, with ls-remote under the same settings.
func TestFetchArgv(t *testing.T) {
	const url = "https://github.com/o/r.git"
	negatives := []string{"^refs/HEAD", "^refs/tags/HEAD", "^refs/heads/HEAD", "^refs/remotes/HEAD", "^refs/remotes/HEAD/HEAD"}
	tests := []struct {
		name  string
		specs []string
		extra []string
		head  bool // ls-remote asks for the remote's HEAD
	}{
		{"outside refs/romeu/origin/", []string{"+refs/heads/*:refs/sandboxes/shop/*"}, nil, false},
		{"a pattern only", []string{"+refs/heads/*:refs/romeu/origin/r/*"}, negatives, false},
		{"a pattern of another prefix", []string{"+refs/foo/*:refs/romeu/origin/r/*"}, append(slices.Clone(negatives), "^refs/foo/HEAD"), false},
		{"two patterns, two prefixes", []string{"+refs/heads/*:refs/romeu/origin/r/*", "+refs/pull/*:refs/romeu/origin/r/pull/*", "+refs/notes/H*:refs/romeu/origin/s/*", "+refs/pull/*:refs/romeu/origin/s/pull/*"}, append(slices.Clone(negatives), "^refs/pull/HEAD", "^refs/notes/HHEAD"), false},
		{"a pattern outside the namespace derives none", []string{"+refs/foo/*:refs/sandboxes/shop/*", "refs/heads/main:refs/romeu/origin/r/main"}, negatives, false},
		{"one branch", []string{"refs/heads/main:refs/romeu/origin/r/pinned", "+refs/heads/*:refs/sandboxes/shop/*"}, negatives, false},
		{"HEAD", []string{"+refs/heads/*:refs/romeu/origin/r/*", "+HEAD:refs/romeu/origin/r/HEAD", "refs/heads/main:refs/romeu/origin/r/pinned"}, negatives, true},
		{"HEAD of two dirs", []string{"+HEAD:refs/romeu/origin/r/HEAD", "HEAD:refs/romeu/origin/s/HEAD"}, negatives, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: "/h", GitHosts: []GitHost{{Host: "github.com", CAFile: "/ca.pem"}}})
			require.NoError(t, err)
			var rec recorder
			rec.install(r)
			advertiseHEAD(r, liveHEAD)
			require.NoError(t, r.Fetch(t.Context(), stubRepo, url, tt.specs))
			require.Len(t, rec.commands, map[bool]int{false: 2, true: 3}[tt.head])

			// The flags of the mode, then the caFile of the host.
			flags := 1 + len(hostFlags) + 2
			resolve := rec.commands[0]
			assert.Equal(t, []string{"ls-remote", "--get-url", url}, resolve.Argv[flags:])
			assert.Equal(t, stubRepo, resolve.Dir)
			fetch := rec.commands[len(rec.commands)-1]
			assert.Equal(t, "http.https://github.com/.sslCAInfo=/ca.pem", fetch.Argv[flags-1])
			if tt.head {
				head := rec.commands[1]
				assert.Equal(t, []string{"ls-remote", "--symref", "--", url, "HEAD"}, head.Argv[flags:])
				assert.Equal(t, fetch.Argv[:flags], head.Argv[:flags], "the flags of the fetch")
				assert.Equal(t, fetch.Env, head.Env)
				assert.Equal(t, stubRepo, head.Dir)
			}

			assert.Equal(t, stubRepo, fetch.Dir)
			assert.Equal(t,
				slices.Concat([]string{"fetch", "--no-tags", "--no-prune", "--no-recurse-submodules", "--no-write-fetch-head", url}, tt.specs, tt.extra),
				fetch.Argv[flags:])
			assert.NotContains(t, fetch.Argv, "origin")
		})
	}

	r, err := New(Config{Mode: Sandbox, Git: "/usr/bin/git", Home: "/h"})
	require.NoError(t, err)
	var rec recorder
	rec.install(r)
	require.NoError(t, r.Fetch(t.Context(), stubRepo, url, []string{"+refs/heads/*:refs/remotes/origin/*"}))
	require.Len(t, rec.commands, 2)
	assert.Equal(t, "+refs/heads/*:refs/remotes/origin/*", rec.commands[1].Argv[len(rec.commands[1].Argv)-1], "a sandbox fetch carries no negative refspec")
}

// TestFetchRefspecChecksRemoteHEAD: a fetch of the remote's HEAD runs
// only when the remote advertises exactly one HEAD that names a
// commit; an unborn HEAD (its ref: line alone), a look-alike such as
// refs/heads/HEAD, and an id that is not one are refused, and the
// fetch never starts.
//
//romeu:invariant I5 test
func TestFetchRefspecChecksRemoteHEAD(t *testing.T) {
	oid := strings.Repeat("a", 40)
	tests := []struct {
		name   string
		advert string
		ok     bool
	}{
		{"live", liveHEAD, true},
		{"detached", oid + "\tHEAD\n", true},
		{"live beside a look-alike", liveHEAD + oid + "\trefs/heads/HEAD\n", true},
		{"sha256", strings.Repeat("0a", 32) + "\tHEAD\n", true},
		{"nothing", "", false},
		{"unborn", "ref: refs/heads/missing\tHEAD\n", false},
		{"a look-alike only", oid + "\trefs/heads/HEAD\n" + oid + "\trefs/tags/HEAD\n", false},
		{"two HEAD lines", liveHEAD + strings.Repeat("b", 40) + "\tHEAD\n", false},
		{"a short id", "abc\tHEAD\n", false},
		{"an upper-case id", strings.Repeat("A", 40) + "\tHEAD\n", false},
		{"no tab", oid + " HEAD\n", false},
		{"a trailing field", oid + "\tHEAD\tx\n", false},
		{"no newline", oid + "\tHEAD", true},
		{"crlf", oid + "\tHEAD\r\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: "/h"})
			require.NoError(t, err)
			var rec recorder
			rec.install(r)
			advertiseHEAD(r, tt.advert)
			err = r.Fetch(t.Context(), stubRepo, "https://github.com/o/r.git",
				[]string{"+HEAD:refs/romeu/origin/app/HEAD", "+refs/heads/*:refs/romeu/origin/app/*"})
			fetched := slices.ContainsFunc(rec.commands, func(c procgroup.Command) bool { return slices.Contains(c.Argv, "fetch") })
			if tt.ok {
				require.NoError(t, err)
				assert.True(t, fetched)
				return
			}
			require.ErrorIs(t, err, ErrRefused)
			assert.Contains(t, err.Error(), "github.com/o/r.git")
			assert.False(t, fetched, "no fetch ran")
		})
	}

	r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: "/h"})
	require.NoError(t, err)
	var rec recorder
	rec.install(r)
	advertiseHEAD(r, strings.Repeat(oid+"\trefs/heads/HEAD\n", outputLimit/len(oid)))
	err = r.Fetch(t.Context(), stubRepo, "https://github.com/o/r.git", []string{"+HEAD:refs/romeu/origin/app/HEAD"})
	require.ErrorIs(t, err, ErrRefused, "an advertisement over the bound")
	assert.Contains(t, err.Error(), "more than")
	assert.Len(t, rec.commands, 2, "no fetch ran")
}

// TestFetchRefspecRemoteHEADMoved: the ask for the remote's HEAD
// follows no redirect either, and fails with ErrMoved as the fetch
// does, before any request reaches the other host.
func TestFetchRefspecRemoteHEADMoved(t *testing.T) {
	url, hosts, reached := moved(t, http.StatusMovedPermanently)
	r, err := New(Config{Mode: Host, Git: gitPath(t), Home: t.TempDir(), GitHosts: hosts})
	require.NoError(t, err)
	dir, _ := newRepo(t)
	err = r.Fetch(t.Context(), dir, url, []string{"+HEAD:refs/romeu/origin/app/HEAD"})
	assertMoved(t, err, url, http.StatusMovedPermanently)
	assert.Contains(t, err.Error(), "git ls-remote", "the ask for HEAD failed, before the fetch")
	assert.Zero(t, reached.Load(), "no request reaches the new host")
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
	specs := []string{"+refs/heads/*:refs/romeu/origin/app/*", "+HEAD:refs/romeu/origin/app/HEAD"}
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

// TestFastForwardKeepsIgnoredFiles: a commit that adds a tracked file
// at the path of an ignored, untracked file of the clone, a file the
// caller's clean check does not see, makes the fast-forward fail,
// naming the path, and leaves the file and HEAD as they were. Plain
// git merge --ff-only overwrites it (the positive control).
func TestFastForwardKeepsIgnoredFiles(t *testing.T) {
	src, _ := newRepo(t)
	dir := filepath.Join(t.TempDir(), "clone")
	plainGit(t, "", "clone", "-q", src, dir)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "info"), 0o700))
	writeFile(t, dir, ".git/info/exclude", "*.env\n")
	writeFile(t, dir, "local.env", "TOKEN=agent\n")
	head := plainGit(t, dir, "rev-parse", "HEAD")
	next := commitFile(t, src, "local.env", "TOKEN=origin\n", "env", "2026-02-01T00:00:00Z")
	plainGit(t, dir, "fetch", "-q", src, "main")

	r := newRunner(t, Sandbox)
	st, err := r.Status(t.Context(), dir)
	require.NoError(t, err)
	require.True(t, st.Clean(), "the caller's check passes: %+v", st)
	err = r.FastForward(t.Context(), dir, next)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local.env")
	assert.Equal(t, head, plainGit(t, dir, "rev-parse", "HEAD"))
	got, err := os.ReadFile(filepath.Join(dir, "local.env"))
	require.NoError(t, err)
	assert.Equal(t, "TOKEN=agent\n", string(got))

	plainGit(t, dir, "merge", "-q", "--ff-only", next)
	got, err = os.ReadFile(filepath.Join(dir, "local.env"))
	require.NoError(t, err)
	assert.Equal(t, "TOKEN=origin\n", string(got), "the plant is live: plain git overwrites it")
}

// filterPlant writes the files a, b, c and d of the repository in dir,
// each named by .gitattributes after a filter driver, and commits them.
// It returns a script that appends a line to the marker it returns and
// passes its input through, and the text of a global config that
// defines the drivers glob and "" (an empty name) with that script as
// their clean and smudge commands, each required.
func filterPlant(t *testing.T, dir string) (script, marker, global string) {
	t.Helper()
	writeFile(t, dir, ".gitattributes", "a filter=evil\nb filter=ev.il\nc filter=glob\nd filter=\n")
	for _, name := range []string{"a", "b", "c", "d"} {
		writeFile(t, dir, name, name+"\n")
	}
	plainGit(t, dir, "add", ".")
	plainGit(t, dir, "commit", "-q", "-m", "files")
	marker = filepath.Join(t.TempDir(), "marker")
	script = filepath.Join(t.TempDir(), "filter.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\necho ran >> '"+marker+"'\ncat\n"), 0o700))
	for _, name := range []string{"glob", ""} {
		global += "[filter \"" + name + "\"]\n\tclean = " + script + "\n\tsmudge = " + script + "\n\trequired = true\n"
	}
	return script, marker, global
}

// plantLocalFilters defines the drivers evil (smudge and clean) and
// ev.il (a long-running process), each required, in the local config of
// the repository in dir.
func plantLocalFilters(t *testing.T, dir, script string) {
	t.Helper()
	plainGit(t, dir, "config", "filter.evil.smudge", script)
	plainGit(t, dir, "config", "filter.evil.clean", script)
	plainGit(t, dir, "config", "filter.evil.required", "true")
	plainGit(t, dir, "config", "filter.ev.il.process", script)
	plainGit(t, dir, "config", "filter.ev.il.required", "true")
}

// TestFastForwardRunsNoSmudgeFilter plants filter drivers in the local
// config of a sandbox clone and in the sandbox's global config, one
// named with a dot, one long-running process and one with an empty
// name, each required and named by .gitattributes, and fast-forwards
// over a commit that changes every file they filter: the checkout runs
// no filter and writes the new content. Plain git runs them (the
// positive control).
//
//romeu:invariant I3 test
func TestFastForwardRunsNoSmudgeFilter(t *testing.T) {
	src, _ := newRepo(t)
	script, marker, global := filterPlant(t, src)
	dir := filepath.Join(t.TempDir(), "clone")
	plainGit(t, "", "clone", "-q", src, dir)
	head := plainGit(t, dir, "rev-parse", "HEAD")
	for _, name := range []string{"a", "b", "c", "d"} {
		writeFile(t, src, name, name+name+"\n")
	}
	plainGit(t, src, "commit", "-q", "-a", "-m", "change")
	next := plainGit(t, src, "rev-parse", "HEAD")
	plainGit(t, dir, "fetch", "-q", src, "main")
	plantLocalFilters(t, dir, script)
	home := t.TempDir()
	writeFile(t, home, ".gitconfig", global)

	r, err := New(Config{Mode: Sandbox, Git: gitPath(t), Home: home})
	require.NoError(t, err)
	require.NoError(t, r.FastForward(t.Context(), dir, next))
	assert.NoFileExists(t, marker)
	assert.Equal(t, next, plainGit(t, dir, "rev-parse", "HEAD"))
	for _, name := range []string{"a", "b", "c", "d"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		assert.Equal(t, name+name+"\n", string(got), name)
	}

	env := []string{"GIT_CONFIG_GLOBAL=" + filepath.Join(home, ".gitconfig")}
	_, _ = plainGitErr(t, dir, env, "reset", "-q", "--hard", head)
	_, _ = plainGitErr(t, dir, env, "merge", "-q", "--ff-only", next)
	assert.FileExists(t, marker, "the plant is live: plain git runs it")
}

// signedCommit writes to the repository in dir a child of its HEAD
// whose gpgsig header holds a fake signature of the given armor (PGP
// or SSH), moves the checked-out branch to it and returns it: enough
// for git to start the signature program of that format.
func signedCommit(t *testing.T, dir, armor string) string {
	t.Helper()
	head := plainGit(t, dir, "rev-parse", "HEAD")
	tree := plainGit(t, dir, "rev-parse", "HEAD^{tree}")
	body := "tree " + tree + "\nparent " + head +
		"\nauthor fixture <fixture@example.invalid> 1767225600 +0000" +
		"\ncommitter fixture <fixture@example.invalid> 1767225600 +0000" +
		"\ngpgsig -----BEGIN " + armor + " SIGNATURE-----\n \n AAAA\n -----END " + armor + " SIGNATURE-----\n\nsigned\n"
	file := filepath.Join(t.TempDir(), "commit")
	require.NoError(t, os.WriteFile(file, []byte(body), 0o600))
	oid := plainGit(t, dir, "hash-object", "-t", "commit", "-w", "--", file)
	plainGit(t, dir, "update-ref", "refs/heads/main", oid)
	return oid
}

// TestFastForwardVerifiesNoSignature: merge.verifySignatures=true in
// the clone's config makes git merge run the signature program of the
// incoming commit's format, gpg.program for a PGP signature and
// gpg.ssh.program for an SSH one (with an allowed signers file set),
// before it fails. FastForward runs
// neither and moves the branch; plain git runs each (the positive
// control).
//
//romeu:invariant I3 test
func TestFastForwardVerifiesNoSignature(t *testing.T) {
	for _, tt := range []struct{ armor, key string }{{"PGP", "gpg.program"}, {"SSH", "gpg.ssh.program"}} {
		t.Run(tt.armor, func(t *testing.T) {
			src, _ := newRepo(t)
			dir := filepath.Join(t.TempDir(), "clone")
			plainGit(t, "", "clone", "-q", src, dir)
			head := plainGit(t, dir, "rev-parse", "HEAD")
			next := signedCommit(t, src, tt.armor)
			plainGit(t, dir, "fetch", "-q", src, "main")
			marker := filepath.Join(t.TempDir(), "marker")
			script := filepath.Join(t.TempDir(), "verify.sh")
			require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\necho ran >> '"+marker+"'\nexit 1\n"), 0o700))
			plainGit(t, dir, "config", "merge.verifySignatures", "true")
			plainGit(t, dir, "config", tt.key, script)
			// git starts gpg.ssh.program only once an allowed signers
			// file is set.
			plainGit(t, dir, "config", "gpg.ssh.allowedSignersFile", filepath.Join(t.TempDir(), "allowed"))

			require.NoError(t, newRunner(t, Sandbox).FastForward(t.Context(), dir, next))
			assert.NoFileExists(t, marker)
			assert.Equal(t, next, plainGit(t, dir, "rev-parse", "HEAD"))

			plainGit(t, dir, "reset", "-q", "--hard", head)
			_, _ = plainGitErr(t, dir, nil, "merge", "-q", "--ff-only", next)
			assert.FileExists(t, marker, "the plant is live: plain git runs it")
		})
	}
}
