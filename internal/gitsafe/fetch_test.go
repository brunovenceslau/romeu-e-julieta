// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"os"
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
