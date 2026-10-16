// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"errors"
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
		{Host, "refs/heads/*:refs/romeu/origin/app/x/*/y", true},
		{Host, "HEAD:refs/romeu/origin/app/HEAD", false},
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
