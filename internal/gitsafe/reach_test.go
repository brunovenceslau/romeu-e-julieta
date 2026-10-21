// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reachFixture makes main base <- one <- two, a side branch base <- s1,
// and two orphan commits o1 and o2 that no ref reaches.
func reachFixture(t *testing.T) (dir string, c map[string]string) {
	t.Helper()
	dir, base := newRepo(t)
	c = map[string]string{"base": base}
	c["one"] = commit(t, dir, "one")
	c["two"] = commit(t, dir, "two")
	plainGit(t, dir, "checkout", "-q", "-b", "side", base)
	c["s1"] = commit(t, dir, "s1")
	tree := plainGit(t, dir, "rev-parse", "HEAD^{tree}")
	c["o1"] = plainGit(t, dir, "commit-tree", "-m", "o1", tree)
	c["o2"] = plainGit(t, dir, "commit-tree", "-m", "o2", tree)
	c["o3"] = plainGit(t, dir, "commit-tree", "-p", c["o2"], "-m", "o3", tree)
	return dir, c
}

// TestUnreachedRealGit answers for a whole set at once which commits
// neither the excluded commits nor another of the set reaches, in the
// order given, each once, with one git rev-list; an exclusion listed
// twice is the same.
func TestUnreachedRealGit(t *testing.T) {
	dir, c := reachFixture(t)
	check := noWrite(t, dir)
	r := newRunner(t, Sandbox)
	got, err := r.Unreached(t.Context(), dir,
		[]string{c["o1"], c["one"], c["s1"], c["base"], c["o2"], c["o1"]},
		[]string{c["two"], c["two"]})
	require.NoError(t, err)
	assert.Equal(t, []string{c["o1"], c["s1"], c["o2"]}, got)

	got, err = r.Unreached(t.Context(), dir, []string{c["o2"], c["o3"], c["one"], c["two"]}, []string{c["base"]})
	require.NoError(t, err)
	assert.Equal(t, []string{c["o3"], c["two"]}, got, "o3 reaches o2, two reaches one")

	got, err = r.Unreached(t.Context(), dir, []string{c["one"]}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{c["one"]}, got, "nothing excluded reaches nothing")

	got, err = r.Unreached(t.Context(), dir, nil, []string{c["two"]})
	require.NoError(t, err)
	assert.Empty(t, got)
	check()
}

// TestBeyondRealGit reports whether the objects the ids reach hold one
// that the excluded ones do not: a new commit, a tag object on a commit
// the exclusion reaches, and a tag of a tree are each beyond.
func TestBeyondRealGit(t *testing.T) {
	dir, c := reachFixture(t)
	r := newRunner(t, Sandbox)
	ok, err := r.Beyond(t.Context(), dir, []string{c["two"], c["one"]}, []string{c["two"]})
	require.NoError(t, err)
	assert.False(t, ok)
	ok, err = r.Beyond(t.Context(), dir, []string{c["two"], c["s1"]}, []string{c["two"]})
	require.NoError(t, err)
	assert.True(t, ok)

	plainGit(t, dir, "tag", "-a", "-m", "at two", "attwo", c["two"])
	ok, err = r.Beyond(t.Context(), dir, []string{plainGit(t, dir, "rev-parse", "refs/tags/attwo")}, []string{c["two"]})
	require.NoError(t, err)
	assert.True(t, ok, "the tag object itself is beyond")
	tree := plainGit(t, dir, "rev-parse", c["s1"]+"^{tree}")
	plainGit(t, dir, "tag", "-a", "-m", "a tree", "tree", tree)
	ok, err = r.Beyond(t.Context(), dir, []string{plainGit(t, dir, "rev-parse", "refs/tags/tree")}, []string{c["two"]})
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = r.Beyond(t.Context(), dir, nil, []string{c["two"]})
	require.NoError(t, err)
	assert.False(t, ok, "no id reaches nothing")
}

// TestReachRefusesBeforeGitStarts: an id that is not a full object id,
// in either list, is refused before git starts, and the argv holds no
// id (they go on the standard input).
func TestReachRefusesBeforeGitStarts(t *testing.T) {
	r := newRunner(t, Sandbox)
	rec := &recorder{}
	rec.install(r)
	full := strings.Repeat("a", 40)
	for _, call := range []func() error{
		func() error { _, err := r.Unreached(context.Background(), stubRepo, []string{"HEAD"}, nil); return err },
		func() error {
			_, err := r.Unreached(context.Background(), stubRepo, []string{full}, []string{"--all"})
			return err
		},
		func() error { _, err := r.Beyond(context.Background(), stubRepo, []string{"main"}, nil); return err },
		func() error {
			_, err := r.Beyond(context.Background(), stubRepo, []string{full}, []string{"^x"})
			return err
		},
	} {
		require.ErrorIs(t, call(), ErrRefused)
	}
	assert.Empty(t, rec.commands)
}

// TestCommonDirRealGit returns the common git dir of the main worktree
// and of a linked one, the same absolute path.
func TestCommonDirRealGit(t *testing.T) {
	dir, _ := newRepo(t)
	wt := t.TempDir() + "/wt"
	plainGit(t, dir, "worktree", "add", "-q", "--detach", wt)
	r := newRunner(t, Sandbox)
	main, err := r.CommonDir(t.Context(), dir)
	require.NoError(t, err)
	linked, err := r.CommonDir(t.Context(), wt)
	require.NoError(t, err)
	assert.Equal(t, main, linked)
	assert.Equal(t, plainGit(t, dir, "rev-parse", "--path-format=absolute", "--git-common-dir"), main)
}

// TestBeyondCountsAFailureAsBeyond: a walk git cannot finish, here an
// exclusion the repository does not hold, answers true, so the caller
// makes the bundle and its failure is the one recorded.
func TestBeyondCountsAFailureAsBeyond(t *testing.T) {
	dir, c := reachFixture(t)
	r := newRunner(t, Sandbox)
	ok, err := r.Beyond(t.Context(), dir, []string{c["two"]}, []string{strings.Repeat("e", 40)})
	require.NoError(t, err)
	assert.True(t, ok)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = r.Beyond(ctx, dir, []string{c["two"]}, []string{c["two"]})
	require.ErrorIs(t, err, context.Canceled, "a context that ended is its error")
}

// TestUnreachedReadsAWideMerge: a merge of 200 parents is one record of
// rev-list --parents, under maxParentsRecord.
func TestUnreachedReadsAWideMerge(t *testing.T) {
	dir, c := reachFixture(t)
	tree := plainGit(t, dir, "rev-parse", c["base"]+"^{tree}")
	args := []string{"commit-tree", "-m", "wide"}
	for i := range 200 {
		args = append(args, "-p", plainGit(t, dir, "commit-tree", "-p", c["base"], "-m", "p"+strconv.Itoa(i), tree))
	}
	wide := plainGit(t, dir, append(args, tree)...)
	got, err := newRunner(t, Sandbox).Unreached(t.Context(), dir, []string{wide}, []string{c["base"]})
	require.NoError(t, err)
	assert.Equal(t, []string{wide}, got)
}
