// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

const (
	generation = "01j9zc2q8x7w4t6y3r5e1v9k0p"
	older      = "01j9zb0000000000000000000a"
)

// pairsOf returns the pairs of p as a map.
func pairsOf(p salvage.Plan) map[string]string {
	m := map[string]string{}
	for _, r := range p.Pairs {
		m[r.Name] = r.OID
	}
	return m
}

// TestPairsNamesWhatASnapshotBundles is REQ-0688 against real git: the
// local branches, tags, notes, the current generation's salvage refs
// and no earlier generation's, each detached worktree HEAD, and each
// reflog entry's commit that no bundled ref reaches, as
// refs/salvage-reflog/<branch>/<n> with n the entry's index in git's
// <branch>@{n}; a commit some bundled ref reaches, a tag among them,
// gets no name.
func TestPairsNamesWhatASnapshotBundles(t *testing.T) {
	dir := repo(t)
	c1 := commit(t, dir, "one")
	c2 := commit(t, dir, "two")
	git(t, dir, "commit", "-q", "--amend", "--allow-empty", "-m", "two amended")
	c2a := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "tag", "-a", "-m", "v1", "v1", c1)
	tagObject := git(t, dir, "rev-parse", "refs/tags/v1")
	git(t, dir, "tag", "light", c1)
	git(t, dir, "notes", "add", "-m", "a note", c1)
	notes := git(t, dir, "rev-parse", "refs/notes/commits")
	git(t, dir, "update-ref", "refs/salvage/"+generation+"/api/stash", c1)
	git(t, dir, "update-ref", "refs/salvage/"+older+"/api/stash", c2)

	// feature loses f1 to a reset: f1 is in feature's reflog only.
	git(t, dir, "branch", "feature", c1)
	git(t, dir, "checkout", "-q", "feature")
	f1 := commit(t, dir, "f1")
	git(t, dir, "reset", "-q", "--hard", c1)
	// A detached commit of the main worktree, left by a checkout: in
	// the HEAD reflog only.
	// A detached commit a tag reaches is bundled by its tag: no name.
	git(t, dir, "checkout", "-q", "--detach", c2a)
	tagged := commit(t, dir, "tagged")
	git(t, dir, "tag", "kept", tagged)
	git(t, dir, "checkout", "-q", "--detach", c2a)
	h1 := commit(t, dir, "h1")
	git(t, dir, "checkout", "-q", "main")
	// A linked worktree whose HEAD is a detached commit no ref reaches.
	wt := filepath.Join(filepath.Dir(dir), "wt")
	git(t, dir, "worktree", "add", "-q", "--detach", wt, c1)
	d1 := commit(t, wt, "d1")

	plan, err := salvage.Pairs(t.Context(), runner(t), dir, generation, nil)
	require.NoError(t, err)
	assert.Empty(t, plan.Skipped)
	h1n := reflogIndex(t, dir, "HEAD", h1)
	want := map[string]string{
		"refs/heads/main":    c2a,
		"refs/heads/feature": c1,
		"refs/tags/v1":       tagObject,
		"refs/tags/light":    c1,
		"refs/tags/kept":     tagged,
		"refs/notes/commits": notes,
		"refs/salvage/" + generation + "/api/stash":                                                  c1,
		"refs/salvage-reflog/main/" + strconv.Itoa(reflogIndex(t, dir, "refs/heads/main", c2)):       c2,
		"refs/salvage-reflog/feature/" + strconv.Itoa(reflogIndex(t, dir, "refs/heads/feature", f1)): f1,
		"refs/salvage-reflog/worktrees/0/" + strconv.Itoa(h1n):                                       h1,
		"refs/salvage-reflog/worktrees/1/HEAD":                                                       d1,
	}
	assert.Equal(t, want, pairsOf(plan))
	for i := 1; i < len(plan.Pairs); i++ {
		assert.Less(t, plan.Pairs[i-1].Name, plan.Pairs[i].Name, "sorted by name")
	}
	assert.Equal(t, plan.Refs(), pairsOf(plan))
}

// TestPairsOfARepositoryWithNoRefIsEmpty: a new repository has no ref
// and an unborn HEAD, so there is nothing to bundle, and no error.
func TestPairsOfARepositoryWithNoRefIsEmpty(t *testing.T) {
	plan, err := salvage.Pairs(t.Context(), runner(t), repo(t), generation, nil)
	require.NoError(t, err)
	assert.Empty(t, plan.Pairs)
	assert.Empty(t, plan.Skipped)
}

// TestPairsSkipsARefusedRefAlone: a tag whose name holds a character
// gitsafe refuses is left out and recorded, escaped, and every other
// ref, the other tags among them, is kept (decided on 2026-10-21).
func TestPairsSkipsARefusedRefAlone(t *testing.T) {
	dir := repo(t)
	c1 := commit(t, dir, "one")
	git(t, dir, "update-ref", "refs/tags/bad\u202etag", c1)
	git(t, dir, "tag", "good", c1)
	plan, err := salvage.Pairs(t.Context(), runner(t), dir, generation, nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"refs/heads/main": c1, "refs/tags/good": c1}, pairsOf(plan))
	require.Len(t, plan.Skipped, 1)
	assert.Equal(t, `refs/tags/bad\u202etag`, plan.Skipped[0].Item)
	assert.ErrorIs(t, plan.Skipped[0].Err, gitsafe.ErrRefused)
}

// TestPairsSkipsABrokenWorktree: a worktree whose directory is now
// another repository has its HEAD reflog left out and recorded, one
// whose .git is gone is prunable and not read, and the snapshot goes on
// (decided while building T060).
func TestPairsSkipsABrokenWorktree(t *testing.T) {
	dir := repo(t)
	c1 := commit(t, dir, "one")
	other := filepath.Join(filepath.Dir(dir), "other")
	gone := filepath.Join(filepath.Dir(dir), "gone")
	git(t, dir, "worktree", "add", "-q", "-b", "o", other)
	git(t, dir, "worktree", "add", "-q", "-b", "g", gone)
	require.NoError(t, os.RemoveAll(other))
	git(t, filepath.Dir(dir), "init", "-q", other)
	require.NoError(t, os.Remove(filepath.Join(gone, ".git")))

	plan, err := salvage.Pairs(t.Context(), runner(t), dir, generation, nil)
	require.NoError(t, err)
	assert.Equal(t, c1, pairsOf(plan)["refs/heads/main"])
	var items []string
	for _, s := range plan.Skipped {
		items = append(items, s.Item)
	}
	assert.Equal(t, []string{"worktree 2"}, items, "other, the second by its id; gone, whose .git is missing, is prunable and not read")
}

// TestPairsNamesAtMostTheCap: the reflog commits beyond
// MaxReflogCommits are left out and recorded; a reflog commit another
// reflog commit reaches needs no name; a base bounds the walk and
// reaches what it reaches.
func TestPairsNamesAtMostTheCap(t *testing.T) {
	dir := repo(t)
	base := commit(t, dir, "base")
	tree := git(t, dir, "rev-parse", "HEAD^{tree}")
	o1 := git(t, dir, "commit-tree", "-m", "o1", tree)
	o2 := git(t, dir, "commit-tree", "-p", o1, "-m", "o2", tree)
	o3 := git(t, dir, "commit-tree", "-m", "o3", tree)
	for _, c := range []string{o1, o2, o3, base} {
		git(t, dir, "update-ref", "-m", "move", "refs/heads/main", c)
	}
	plan, err := salvage.Pairs(t.Context(), runner(t), dir, generation, []string{base})
	require.NoError(t, err)
	assert.Empty(t, plan.Skipped)
	names := map[string]string{}
	for name, oid := range pairsOf(plan) {
		names[oid] = name
	}
	assert.Contains(t, names, o2)
	assert.Contains(t, names, o3)
	assert.NotContains(t, names, o1, "o2 reaches it")

	old := salvage.MaxReflogCommits
	salvage.MaxReflogCommits = 1
	t.Cleanup(func() { salvage.MaxReflogCommits = old })
	plan, err = salvage.Pairs(t.Context(), runner(t), dir, generation, []string{base})
	require.NoError(t, err)
	assert.Len(t, plan.Pairs, 2, "main and one reflog commit")
	require.Len(t, plan.Skipped, 1)
	assert.Equal(t, "1 reflog commits", plan.Skipped[0].Item)
}
