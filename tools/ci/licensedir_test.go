// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGitCommonDir shows that gitCommonDir names the git directory by an
// absolute path in a clone and in a linked worktree. Git alone answers
// relatively in the main working tree (".git"); a linked worktree
// already gets an absolute path.
func TestGitCommonDir(t *testing.T) {
	r := newTree(t)
	r.Commit("base")
	want, err := filepath.EvalSymlinks(filepath.Join(r.Dir, ".git"))
	require.NoError(t, err)

	got, err := gitCommonDir(t.Context(), r.Dir)
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(got), "an absolute path: %s", got)
	gotReal, err := filepath.EvalSymlinks(got)
	require.NoError(t, err)
	assert.Equal(t, want, gotReal, "in a clone")

	linked := filepath.Join(t.TempDir(), "linked")
	r.Git("worktree", "add", "--quiet", "--detach", linked)
	got, err = gitCommonDir(t.Context(), linked)
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(got), "an absolute path: %s", got)
	gotReal, err = filepath.EvalSymlinks(got)
	require.NoError(t, err)
	assert.Equal(t, want, gotReal, "in a linked worktree, the main clone's git directory")
}

// TestAllStepsWithoutGitDirectory shows what allSteps does when git
// cannot name the git directory, as in a tree that is no repository: on
// linux the license step cannot run and says why, and on another
// platform its skip stands, with no error, since the step does not run
// there at all.
func TestAllStepsWithoutGitDirectory(t *testing.T) {
	root := t.TempDir()
	// A temp directory inside a repository would be part of it; git stops
	// looking at the parent of the fixture.
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	t.Setenv("LC_ALL", "C") // git's message is then the untranslated one
	_, err := gitCommonDir(t.Context(), root)
	require.ErrorContains(t, err, "not a git repository", "the fixture is no repository")
	var license step
	for _, s := range allSteps(t.Context(), root, lintTools{}, filepath.Join(root, "cover.out")) {
		if s.name == "license" {
			license = s
		}
	}
	require.Equal(t, "license", license.name)
	if runtime.GOOS == "linux" {
		require.Error(t, license.unavailable, "a git directory that is not known fails the step")
		assert.Empty(t, license.skip)
		return
	}
	require.NoError(t, license.unavailable, "a step that is skipped is not unavailable as well")
	assert.NotEmpty(t, license.skip)
}

// TestLicenseStepUnnameablePaths shows that the root and the git
// directory are each refused when they hold a comma or an equals sign,
// which the bind mount option cannot name.
func TestLicenseStepUnnameablePaths(t *testing.T) {
	for _, tt := range []struct {
		name, root, gitDir string
		refused            bool
	}{
		{"plain paths", "/src/repo", "/src/main/.git", false},
		{"a comma in the root", "/src/a,b", "/src/main/.git", true},
		{"an equals sign in the root", "/src/a=b", "/src/main/.git", true},
		{"a comma in the git directory", "/src/repo", "/src/a,b/.git", true},
		{"an equals sign in the git directory", "/src/repo", "/src/a=b/.git", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := licenseStep(tt.root, tt.gitDir, "linux", nil)
			if tt.refused {
				require.ErrorContains(t, s.unavailable, "the bind mount")
				return
			}
			require.NoError(t, s.unavailable)
		})
	}
}
