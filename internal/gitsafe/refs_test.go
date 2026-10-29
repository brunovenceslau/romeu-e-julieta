// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// refNameCases are the ref-name checks of git check-ref-format, plus
// gitsafe's own rule that a name starts with refs/. They seed the fuzz
// target too.
var refNameCases = []struct {
	name    string
	ok      bool
	pattern bool // checked as a refspec side, where one * is allowed
}{
	{"refs/heads/main", true, false},
	{"refs/romeu/origin/app/feature/x-1", true, false},
	{"refs/heads/caf\u00e9", true, false},
	{"refs/heads/*", true, true},
	{"refs/heads/feat*", true, true},
	{"refs/heads/*", false, false},
	{"refs/*/x/*", false, true},
	{"refs/heads/a*b*", false, true},
	{"", false, false},
	{"refs", false, false},
	{"refs/", false, false},
	{"HEAD", false, false},
	{"heads/main", false, false},
	{"-refs/heads/main", false, false},
	{"refs/heads/main/", false, false},
	{"refs//heads", false, false},
	{"refs/heads/.hidden", false, false},
	{"refs/heads/a..b", false, false},
	{"refs/heads/x.lock", false, false},
	{"refs/heads/x.lock/y", false, false},
	{"refs/heads/x.", false, false},
	{"refs/heads/a@{1}", false, false},
	{"refs/heads/@", true, false},
	{"refs/heads/a b", false, false},
	{"refs/heads/a\tb", false, false},
	{"refs/heads/a\x7fb", false, false},
	{"refs/heads/a\x00b", false, false},
	{"refs/heads/a~1", false, false},
	{"refs/heads/a^", false, false},
	{"refs/heads/a:b", false, false},
	{"refs/heads/a?", false, false},
	{"refs/heads/a[b", false, false},
	{"refs/heads/a\\b", false, false},
	{"refs/heads/\xff", false, false},
}

// TestCheckRefName is the table of the ref-name checks (10 10.1).
func TestCheckRefName(t *testing.T) {
	for _, tt := range refNameCases {
		err := checkRefName(tt.name, tt.pattern)
		if tt.ok {
			assert.NoError(t, err, "%q", tt.name)
		} else {
			assert.ErrorIs(t, err, ErrRefused, "%q", tt.name)
		}
	}
}

// FuzzCheckRefName fuzzes the ref-name checks, seeded with the table
// above: a name they accept is one git accepts too, and they never
// panic.
func FuzzCheckRefName(f *testing.F) {
	for _, tt := range refNameCases {
		f.Add(tt.name, tt.pattern)
	}
	f.Fuzz(func(t *testing.T, name string, pattern bool) {
		if checkRefName(name, pattern) != nil {
			return
		}
		args := []string{"check-ref-format"}
		if pattern {
			args = append(args, "--refspec-pattern")
		}
		_, err := plainGitErr(t, t.TempDir(), nil, append(args, name)...)
		require.NoError(t, err, "gitsafe accepts %q and git does not", name)
	})
}

// TestCheckRefspec is the refspec validator table (I5): a fetch writes
// into the allowlisted prefixes of its mode only, never refs/heads,
// refs/tags, refs/remotes or HEAD on the host, and never into a
// create-only namespace.
//
//romeu:invariant I5 test
func TestCheckRefspec(t *testing.T) {
	tests := []struct {
		mode Mode
		spec string
		ok   bool
	}{
		{Host, "+refs/heads/*:refs/romeu/origin/app/*", true},
		{Host, "refs/heads/main:refs/romeu/origin/app/main", true},
		{Host, "+refs/heads/*:refs/sandboxes/shop/*", true},
		{Host, "+refs/heads/*:refs/romeu/snapshots/shop/*", true},
		{Host, "+refs/heads/*:refs/heads/*", false},
		{Host, "refs/heads/main:refs/heads/main", false},
		{Host, "refs/tags/v1:refs/tags/v1", false},
		{Host, "+refs/heads/*:refs/remotes/origin/*", false},
		{Host, "refs/heads/main:HEAD", false},
		{Host, "refs/heads/main", false},
		{Host, "refs/heads/main:", false},
		{Host, ":refs/romeu/origin/app/main", false},
		{Host, "^refs/heads/main", false},
		{Host, "refs/heads/main:refs/romeu/salvage/shop/g/s/app/main", false},
		{Host, "refs/heads/main:refs/romeu/base/shop/g/app", false},
		{Host, "refs/heads/main:refs/romeu/origin/app", false},
		{Host, "refs/heads/main:refs/romeu/origin/", false},
		{Host, "refs/heads/main:refs/romeu/origin//main", false},
		{Host, "refs/heads/main:refs/romeu/originx/app/main", false},
		{Host, "refs/heads/main:refs/romeu/origin/app/*", false},
		{Host, "refs/heads/*:refs/romeu/origin/app/main", false},
		{Host, "refs/heads/*:refs/romeu/origin/*/x", false},
		{Host, "refs/heads/*:refs/romeu/origin/app/x/*/y", false},
		{Host, "refs/heads/*:refs/romeu/origin/app/x/*", true},
		{Host, "+refs/heads/H*:refs/romeu/origin/app/*", true},
		{Host, "+refs/heads/*:refs/romeu/origin/app/HE*", false},
		{Host, "+refs/heads/*:refs/romeu/origin/app/x/a*", false},
		{Host, "+refs/heads/*:refs/sandboxes/shop/H*", true},
		{Host, "HEAD:refs/romeu/origin/app/HEAD", true},
		{Host, "+HEAD:refs/romeu/origin/app/HEAD", true},
		{Host, "HEAD:refs/romeu/origin/app/main", false},
		{Host, "HEAD:refs/romeu/origin/app/x/HEAD", false},
		{Host, "HEAD:refs/romeu/origin/HEAD", false},
		{Host, "HEAD:refs/romeu/origin//HEAD", false},
		{Host, "HEAD:refs/romeu/origin/*/HEAD", false},
		{Host, "HEAD:refs/romeu/origin/app/*", false},
		{Host, "HEAD:refs/sandboxes/shop/HEAD", false},
		{Host, "HEAD:refs/romeu/snapshots/shop/HEAD", false},
		{Host, "HEAD:HEAD", false},
		{Host, "HEAD:refs/heads/HEAD", false},
		{Host, "head:refs/romeu/origin/app/HEAD", false},
		{Host, "FETCH_HEAD:refs/romeu/origin/app/HEAD", false},
		{Host, "ORIG_HEAD:refs/romeu/origin/app/HEAD", false},
		{Host, "HEAD*:refs/romeu/origin/app/*", false},
		{Host, "++HEAD:refs/romeu/origin/app/HEAD", false},
		{Host, "^HEAD:refs/romeu/origin/app/HEAD", false},
		{Host, "^refs/heads/HEAD", false},
		{Host, ":refs/romeu/origin/app/HEAD", false},
		{Host, "HEAD:", false},
		{Host, "HEAD", false},
		{Host, "+refs/heads/H*:refs/romeu/origin/app/H*", false},
		{Host, "+refs/heads/*D:refs/romeu/origin/app/*D", false},
		{Host, "+refs/heads/*:refs/romeu/origin/app/H*", false},
		{Host, "+refs/heads/*:refs/romeu/origin/app/*D", false},
		{Host, "refs/heads/HEAD:refs/romeu/origin/app/HEAD", false},
		{Host, "refs/heads/main:refs/romeu/origin/app/HEAD", false},
		{Host, "+refs/heads/main:refs/romeu/origin/app/HEAD", false},
		{Host, "refs/heads/main:refs/romeu/origin/app/x/HEAD", true},
		{Host, "refs/heads/main:refs/romeu/origin/app/head", false},
		{Host, "+refs/heads/main:refs/romeu/origin/app/Head", false},
		{Host, "HEAD:refs/romeu/origin/app/head", false},
		{Host, "refs/heads/main:refs/romeu/origin/app/heads", true},
		{Host, "refs/heads/main:refs/sandboxes/shop/HEAD", true},
		{Host, "-x:refs/romeu/origin/app/main", false},
		{Host, "refs/heads/a..b:refs/romeu/origin/app/main", false},
		{Host, "refs/heads/main:refs/romeu/origin/app/main:x", false},
		{Sandbox, "+refs/heads/*:refs/remotes/origin/*", true},
		{Sandbox, "refs/heads/main:refs/remotes/origin/main", true},
		{Sandbox, "+refs/heads/*:refs/heads/*", false},
		{Sandbox, "refs/heads/main:HEAD", false},
		{Sandbox, "refs/tags/v1:refs/tags/v1", false},
		{Sandbox, "+refs/heads/*:refs/romeu/origin/app/*", false},
		{Sandbox, "refs/heads/main:refs/salvage/g/s/stash-0", false},
		{Sandbox, "refs/heads/main:refs/remotes/origin", false},
		{Sandbox, "HEAD:refs/remotes/origin/HEAD", false},
		{Sandbox, "HEAD:refs/romeu/origin/app/HEAD", false},
	}
	for _, tt := range tests {
		t.Run(tt.mode.String()+"/"+tt.spec, func(t *testing.T) {
			err := checkRefspec(tt.mode, tt.spec)
			if tt.ok {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, ErrRefused)
			}
		})
	}
}

// TestFetchRefusesRefspec: a fetch with no refspec, or with one the
// table refuses, runs nothing.
func TestFetchRefusesRefspec(t *testing.T) {
	r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: "/h"})
	require.NoError(t, err)
	var rec recorder
	rec.install(r)
	require.ErrorIs(t, r.Fetch(t.Context(), stubRepo, "https://github.com/o/r.git", nil), ErrRefused)
	require.ErrorIs(t, r.Fetch(t.Context(), stubRepo, "https://github.com/o/r.git",
		[]string{"refs/heads/main:refs/romeu/origin/r/main", "+refs/heads/*:refs/heads/*"}), ErrRefused)
	assert.Empty(t, rec.commands)
}

// TestFetchRefspecRemoteHEAD fetches, over the loopback daemon, the
// remote's HEAD into refs/romeu/origin/<dir>/HEAD with the branches
// beside it: the ref is the commit the remote's HEAD names, not the
// clone's own refs/remotes/origin/HEAD (I9), and nothing outside the
// namespace moves. A ref of the remote that git would read HEAD as, a
// branch or a tag named HEAD among them, never lands in <dir>/HEAD,
// and an origin whose HEAD is unborn is refused before any fetch runs,
// so nothing is written and a <dir>/HEAD of an earlier fetch stays.
//
//romeu:invariant I5 test
func TestFetchRefspecRemoteHEAD(t *testing.T) {
	const (
		head     = "+HEAD:refs/romeu/origin/app/HEAD"
		branches = "+refs/heads/*:refs/romeu/origin/app/*"
		tags     = "+refs/tags/*:refs/romeu/origin/app/tags/*"
	)
	tests := []struct {
		name    string
		specs   []string
		planted string // a ref of the remote git would read HEAD as
		unborn  bool   // the remote's HEAD names a missing branch
		stale   bool   // <dir>/HEAD holds a commit of an earlier fetch
	}{
		{name: "head first", specs: []string{head, branches}},
		{name: "branches first", specs: []string{branches, head}},
		{name: "head alone", specs: []string{head}},
		{name: "a pattern only, a branch named HEAD", specs: []string{branches}, planted: "refs/heads/HEAD"},
		{name: "a pattern only, a tag named HEAD", specs: []string{branches, tags}, planted: "refs/tags/HEAD"},
		{name: "a pattern of another prefix, its HEAD", specs: []string{branches, "+refs/foo/*:refs/romeu/origin/app/*"}, planted: "refs/foo/HEAD"},
		{name: "a pattern of another prefix, its HEAD, head too", specs: []string{head, "+refs/foo/*:refs/romeu/origin/app/*", branches}, planted: "refs/foo/HEAD"},
		{name: "a branch named HEAD, head first", specs: []string{head, branches}, planted: "refs/heads/HEAD"},
		{name: "a branch named HEAD, branches first", specs: []string{branches, head}, planted: "refs/heads/HEAD"},
		{name: "a tag named HEAD", specs: []string{head, branches, tags}, planted: "refs/tags/HEAD"},
		{name: "refs/HEAD", specs: []string{head, branches}, planted: "refs/HEAD"},
		{name: "refs/remotes/HEAD", specs: []string{head, branches}, planted: "refs/remotes/HEAD"},
		{name: "refs/remotes/HEAD/HEAD", specs: []string{head, branches}, planted: "refs/remotes/HEAD/HEAD"},
		{name: "unborn", specs: []string{head, branches}, unborn: true},
		{name: "unborn, head alone", specs: []string{head}, unborn: true},
		{name: "unborn, a stale HEAD", specs: []string{head, branches}, unborn: true, stale: true},
		{name: "unborn, a branch named HEAD", specs: []string{head, branches}, planted: "refs/heads/HEAD", unborn: true},
		{name: "unborn, a branch named HEAD, a stale HEAD", specs: []string{head, branches}, planted: "refs/heads/HEAD", unborn: true, stale: true},
		{name: "unborn, a tag named HEAD", specs: []string{head, tags}, planted: "refs/tags/HEAD", unborn: true},
		{name: "unborn, refs/HEAD", specs: []string{head}, planted: "refs/HEAD", unborn: true},
		{name: "unborn, refs/remotes/HEAD", specs: []string{head}, planted: "refs/remotes/HEAD", unborn: true},
		{name: "unborn, refs/remotes/HEAD/HEAD", specs: []string{head}, planted: "refs/remotes/HEAD/HEAD", unborn: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, one := newRepo(t)
			two := commit(t, src, "two")
			plainGit(t, src, "checkout", "-q", "-b", "planted")
			three := commit(t, src, "three")
			plainGit(t, src, "checkout", "-q", "main")
			plainGit(t, src, "branch", "side", one)
			base := bareCopy(t, src, "repo.git")
			bare := filepath.Join(base, "repo.git")
			if tt.planted != "" {
				plainGit(t, bare, "update-ref", tt.planted, three)
			}
			if strings.HasPrefix(tt.planted, "refs/foo/") {
				plainGit(t, bare, "update-ref", "refs/foo/other", two)
			}
			plainGit(t, bare, "update-ref", "-d", "refs/heads/planted")
			plainGit(t, bare, "symbolic-ref", "HEAD", map[bool]string{false: "refs/heads/side", true: "refs/heads/missing"}[tt.unborn])
			url := serveGit(t, base) + "repo.git"

			dir := filepath.Join(t.TempDir(), "clone")
			plainGit(t, src, "clone", "-q", src, dir)
			plainGit(t, dir, "update-ref", "--no-deref", "refs/remotes/origin/HEAD", two)
			outside := func() string {
				return plainGit(t, dir, "for-each-ref", "--format=%(refname) %(objectname)", "--exclude=refs/romeu/origin/") +
					plainGit(t, dir, "rev-parse", "HEAD")
			}
			if tt.stale {
				plainGit(t, dir, "update-ref", "refs/romeu/origin/app/HEAD", two)
			}
			before := outside()
			staleRefs := plainGit(t, dir, "for-each-ref", "--format=%(refname) %(objectname)", "refs/romeu/origin/")
			r := newRunner(t, Host)
			fetchRan := false
			run := r.start
			r.start = func(ctx context.Context, c procgroup.Command) error {
				fetchRan = fetchRan || slices.Contains(c.Argv, "fetch")
				return run(ctx, c)
			}
			err := r.Fetch(t.Context(), dir, url, tt.specs)
			assert.Equal(t, before, outside(), "nothing outside the namespace moved")
			fetched := plainGit(t, dir, "for-each-ref", "--format=%(refname) %(objectname)", "refs/romeu/origin/")
			assert.NotContains(t, fetched, three, "the planted ref landed in the namespace")
			if tt.unborn {
				require.ErrorIs(t, err, ErrRefused)
				assert.Contains(t, err.Error(), url)
				assert.False(t, fetchRan, "refused before any fetch ran")
				assert.Equal(t, staleRefs, fetched, "nothing written")
				return
			}
			require.NoError(t, err)
			assert.True(t, fetchRan)
			if strings.HasPrefix(tt.planted, "refs/foo/") {
				assert.Equal(t, two, refOf(t, dir, "refs/romeu/origin/app/other"), "the pattern of refs/foo/ fetched")
			}
			if !slices.Contains(tt.specs, head) {
				assert.Empty(t, refOf(t, dir, "refs/romeu/origin/app/HEAD"), "no fetch of the remote's HEAD, no <dir>/HEAD")
				assert.Equal(t, two, refOf(t, dir, "refs/romeu/origin/app/main"))
				return
			}
			assert.Equal(t, one, refOf(t, dir, "refs/romeu/origin/app/HEAD"), "the remote's HEAD, not refs/remotes/origin/HEAD")
			if len(tt.specs) > 1 {
				assert.Equal(t, two, refOf(t, dir, "refs/romeu/origin/app/main"))
				assert.Equal(t, one, refOf(t, dir, "refs/romeu/origin/app/side"))
			}
		})
	}
}

// TestCreateRef: a create-only ref is created once, and a second
// create on the same ref fails and leaves it where it was (I6).
//
//romeu:invariant I6 test
func TestCreateRef(t *testing.T) {
	for _, tt := range []struct {
		mode Mode
		ref  string
	}{
		{Host, "refs/romeu/salvage/shop/01j9/01ja/app/main"},
		{Host, "refs/romeu/base/shop/01j9/app"},
		{Sandbox, "refs/salvage/01j9/01ja/stash-0"},
	} {
		t.Run(tt.mode.String()+"/"+tt.ref, func(t *testing.T) {
			dir, first := newRepo(t)
			second := commit(t, dir, "two")
			r := newRunner(t, tt.mode)
			require.NoError(t, r.CreateRef(t.Context(), dir, tt.ref, first))
			assert.Equal(t, first, refOf(t, dir, tt.ref))

			err := r.CreateRef(t.Context(), dir, tt.ref, second)
			require.Error(t, err)
			_, isExit := errors.AsType[*procgroup.ExitError](err)
			assert.True(t, isExit, "git refused it: %v", err)
			assert.Equal(t, first, refOf(t, dir, tt.ref), "the ref did not move")
			require.Error(t, r.CreateRef(t.Context(), dir, tt.ref, first), "not even to the same commit")
		})
	}
}

// TestCreateRefRefuses: CreateRef writes only into the create-only
// namespaces of its mode, and only a full object id.
//
//romeu:invariant I6 test
func TestCreateRefRefuses(t *testing.T) {
	oid := strings.Repeat("c", 40)
	tests := []struct {
		mode Mode
		ref  string
		oid  string
	}{
		{Host, "refs/heads/main", oid},
		{Host, "refs/romeu/origin/app/main", oid},
		{Host, "refs/sandboxes/shop/main", oid},
		{Host, "refs/romeu/salvage/shop", oid},
		{Host, "refs/romeu/salvage/", oid},
		{Host, "refs/salvage/g/s/stash-0", oid},
		{Host, "HEAD", oid},
		{Host, "refs/romeu/base/shop/g/app", "main"},
		{Host, "refs/romeu/base/shop/g/app", "abc"},
		{Host, "refs/romeu/base/shop/g/app", strings.Repeat("g", 40)},
		{Host, "refs/romeu/base/shop/g/app", strings.Repeat("A", 40)},
		{Host, "refs/romeu/base/shop/g/app", "-" + oid[1:]},
		{Host, "refs/romeu/base/shop/g/*", oid},
		{Sandbox, "refs/romeu/salvage/shop/g/s/app/main", oid},
		{Sandbox, "refs/heads/main", oid},
	}
	for _, tt := range tests {
		r, err := New(Config{Mode: tt.mode, Git: "/usr/bin/git", Home: "/h"})
		require.NoError(t, err)
		var rec recorder
		rec.install(r)
		require.ErrorIs(t, r.CreateRef(t.Context(), stubRepo, tt.ref, tt.oid), ErrRefused, "%s %s %s", tt.mode, tt.ref, tt.oid)
		assert.Empty(t, rec.commands)
	}
	r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: "/h"})
	require.NoError(t, err)
	var rec recorder
	rec.install(r)
	require.NoError(t, r.CreateRef(t.Context(), stubRepo, "refs/romeu/base/shop/g/app", strings.Repeat("0a", 32)), "a sha256 id")
}

// TestNamespacesDisjoint: no create-only namespace is a fetch
// destination, in either mode (I6: never a fetch destination).
func TestNamespacesDisjoint(t *testing.T) {
	for _, mode := range []Mode{Host, Sandbox} {
		for _, created := range createOnly[mode] {
			for _, fetched := range fetchDestinations[mode] {
				assert.False(t, strings.HasPrefix(created, fetched) || strings.HasPrefix(fetched, created),
					"%s and %s overlap", created, fetched)
			}
		}
	}
}
