// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package status_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/status"
)

// TestStatusHelp is the help test of julieta status (12 12.3): its help
// exits 0 and opens with its usage line.
func TestStatusHelp(t *testing.T) {
	code, stdout, stderr := julieta(t.Context(), "status", "--help")
	assert.Equal(t, 0, code)
	assert.Empty(t, stderr)
	assert.True(t, strings.HasPrefix(stdout, "Usage: julieta status [flags]\n"), stdout)
}

// TestStatusWithoutManifest exits 2 with manifest-protocol when
// JULIETA_MANIFEST is unset and none is cached (04 4.3, REQ-0425).
func TestStatusWithoutManifest(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(setup.ManifestEnv, "")
	code, stdout, stderr := julieta(t.Context(), "status", "--json")
	assert.Equal(t, 2, code)
	assert.Empty(t, stdout)
	assert.Equal(t, "RJ-342", errorID(t, stderr))
}

// TestStatusAfterSetup reports, after a setup that moved the primary's
// default branch, the manifest's project, spec commit and egress set,
// each repo's branch, HEAD, branch heads with ahead and behind, worktree
// and clean state, and the warning default-branch-moved (04 4.3,
// REQ-0424; 07 7.5).
func TestStatusAfterSetup(t *testing.T) {
	s := newSandbox(t)
	old := git(t, s.primary(), "rev-parse", "HEAD")
	tip := push(t, filepath.Join(s.base, "api.git"), "two")
	code, _, stderr := julieta(t.Context(), "setup")
	require.Equal(t, 0, code, stderr)

	rep := statusJSON(t)
	assert.Equal(t, "shop", rep.Project)
	assert.Equal(t, s.m.SpecCommit, rep.SpecCommit)
	assert.Equal(t, []string{"dl.google.com", "github.com"}, rep.Egress)
	assert.Empty(t, rep.Errors)
	assert.Equal(t, []setup.Warning{{Slug: setup.DefaultBranchMoved, Message: "api " + old + ".." + tip}}, rep.Warnings)
	require.Len(t, rep.Repos, 2)
	api := rep.Repos[0]
	assert.Equal(t, status.Repo{
		Dir: "api", Path: s.primary(), Branch: "main", Head: tip,
		Branches:  []status.Branch{{Name: "main", Head: tip, Current: true, Upstream: "refs/remotes/origin/main"}},
		Dirty:     []string{},
		Errors:    []string{},
		Stashes:   []status.Stash{},
		Worktrees: []status.Worktree{{Path: api.Worktrees[0].Path, Head: tip, Branch: "refs/heads/main"}},
	}, api)
	assert.Equal(t, "web", rep.Repos[1].Dir)
	assert.Empty(t, rep.Repos[1].Errors)
}

// TestStatusOfAWorkingTree reports what a working tree holds: the
// kinds of change, a stash, a branch ahead of its upstream and one
// whose upstream is gone, and the repo-local core.hooksPath that
// disables snapshots.
func TestStatusOfAWorkingTree(t *testing.T) {
	s := newSandbox(t)
	dir := s.primary()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "stashed"), []byte("a"), 0o600))
	git(t, dir, "add", "stashed")
	git(t, dir, "stash", "push", "-q", "-m", "wip")
	commit(t, dir, "mine")
	git(t, dir, "branch", "-q", "topic")
	git(t, dir, "config", "branch.topic.remote", ".")
	git(t, dir, "config", "branch.topic.merge", "refs/heads/gone")
	git(t, dir, "config", "core.hooksPath", ".githooks")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "new"), nil, 0o600))

	rep := statusJSON(t)
	api := rep.Repos[0]
	assert.Empty(t, api.Errors)
	assert.Equal(t, []string{"untracked"}, api.Dirty)
	require.Len(t, api.Stashes, 1)
	assert.Contains(t, api.Stashes[0].Subject, "wip")
	require.Len(t, api.Branches, 2)
	assert.Equal(t, 1, api.Branches[0].Ahead, "main is one ahead of origin/main")
	assert.Equal(t, 0, api.Branches[0].Behind)
	assert.Equal(t, "topic", api.Branches[1].Name)
	assert.True(t, api.Branches[1].UpstreamGone)
	assert.True(t, api.SnapshotsDisabled)
	assert.Equal(t, ".githooks", api.HooksPath)
}

// TestStatusGoesOnPastAnUnreadableRepo lists the reads of a repo that
// failed, the missing secondary before setup clones it, and reports the
// others.
func TestStatusGoesOnPastAnUnreadableRepo(t *testing.T) {
	newSandbox(t)
	rep := statusJSON(t)
	require.Len(t, rep.Repos, 2)
	assert.Empty(t, rep.Repos[0].Errors)
	assert.Equal(t, "main", rep.Repos[0].Branch)
	assert.NotEmpty(t, rep.Repos[1].Errors)
}

// TestStatusShowsAFailedFetch shows the warning fetch-failed setup
// recorded, on stderr in the text form, as setup prints it.
func TestStatusShowsAFailedFetch(t *testing.T) {
	s := newSandbox(t)
	s.m.Repos[0].URL = s.url + "gone.git"
	s.setManifest(t)
	code, _, stderr := julieta(t.Context(), "setup")
	require.Equal(t, 0, code, stderr)
	code, stdout, stderr := julieta(t.Context(), "status")
	require.Equal(t, 0, code, stderr)
	assert.True(t, strings.HasPrefix(stderr, "warning fetch-failed: api: "), stderr)
	assert.NotContains(t, stdout, "warning")
	assert.True(t, strings.HasPrefix(stdout, "project  shop\nspec     "+s.m.SpecCommit+"\negress   dl.google.com github.com\nrepo api  "+s.primary()+"\n"), stdout)
	assert.Contains(t, stdout, "  branch     main ")
	assert.Contains(t, stdout, "  dirty      clean\n")
	assert.Contains(t, stdout, "  snapshots  enabled\n")
}

// TestStatusWritesNothing leaves HOME as it found it.
func TestStatusWritesNothing(t *testing.T) {
	s := newSandbox(t)
	before := tree(t, s.home)
	statusJSON(t)
	assert.Equal(t, before, tree(t, s.home))
}

// tree lists the paths below dir.
func tree(t *testing.T, dir string) []string {
	t.Helper()
	var paths []string
	require.NoError(t, filepath.WalkDir(dir, func(p string, _ os.DirEntry, err error) error {
		paths = append(paths, p)
		return err
	}))
	return paths
}

// TestDecodeIsStrict reads the --json document strictly: the fixtures
// with a duplicate member and with an unknown one are refused, and the
// valid one is read.
func TestDecodeIsStrict(t *testing.T) {
	for name, valid := range map[string]bool{"valid.json": true, "invalid-duplicate-member.json": false, "invalid-unknown-member.json": false} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		require.NoError(t, err)
		_, err = status.Decode(data)
		if valid {
			require.NoError(t, err, name)
		} else {
			require.Error(t, err, name)
		}
	}
}
