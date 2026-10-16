// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package license

import (
	"bytes"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGitCommonDir shows that GitCommonDir names the git directory by an
// absolute path in a clone and in a linked worktree. Git alone answers
// relatively in the main working tree (".git"); a linked worktree
// already gets an absolute path.
func TestGitCommonDir(t *testing.T) {
	r := citest.NewTree(t)
	r.Commit("base")
	want, err := filepath.EvalSymlinks(filepath.Join(r.Dir, ".git"))
	require.NoError(t, err)

	got, err := GitCommonDir(t.Context(), r.Dir)
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(got), "an absolute path: %s", got)
	gotReal, err := filepath.EvalSymlinks(got)
	require.NoError(t, err)
	assert.Equal(t, want, gotReal, "in a clone")

	linked := filepath.Join(t.TempDir(), "linked")
	r.Git("worktree", "add", "--quiet", "--detach", linked)
	got, err = GitCommonDir(t.Context(), linked)
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
	_, err := GitCommonDir(t.Context(), root)
	require.ErrorContains(t, err, "not a git repository", "the fixture is no repository")
	var license check.Step
	for _, s := range Steps(t.Context(), check.Plan{Root: root}) {
		if s.Name == "license" {
			license = s
		}
	}
	require.Equal(t, "license", license.Name)
	if runtime.GOOS == "linux" {
		require.Error(t, license.Unavailable, "a git directory that is not known fails the step")
		assert.Empty(t, license.Skip)
		return
	}
	require.NoError(t, license.Unavailable, "a step that is skipped is not unavailable as well")
	assert.NotEmpty(t, license.Skip)
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
			s := Step(tt.root, tt.gitDir, "linux", nil)
			if tt.refused {
				require.ErrorContains(t, s.Unavailable, "the bind mount")
				return
			}
			require.NoError(t, s.Unavailable)
		})
	}
}

// TestLicenseStep pins the license step: reuse from the image pinned
// by digest, on linux, with the tree mounted read-only, no network and
// no privilege; on another platform a step that is not run, which all
// reports with a "--" line, and neither "ok" nor "FAIL".
func TestLicenseStep(t *testing.T) {
	env := []string{"PATH=/x"}
	assert.Equal(t, check.Step{
		Name: "license",
		Argv: []string{"docker", "run", "--rm", "--network", "none",
			"--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--read-only", "--user", "65534",
			"--env", "GIT_CONFIG_COUNT=2",
			"--env", "GIT_CONFIG_KEY_0=safe.directory", "--env", "GIT_CONFIG_VALUE_0=/src/repo",
			"--env", "GIT_CONFIG_KEY_1=safe.directory", "--env", "GIT_CONFIG_VALUE_1=/src/main/.git",
			"--mount", "type=bind,source=/src/repo,target=/src/repo,readonly",
			"--mount", "type=bind,source=/src/main/.git,target=/src/main/.git,readonly",
			"--workdir", "/src/repo",
			"fsfe/reuse:6.2.0@sha256:85462a75c0f8efda09ddd190b92816b70e7662577c8427429e11e1b9f25a992e", "lint"},
		Environ: env,
	}, Step("/src/repo", "/src/main/.git", "linux", env))
	assert.Regexp(t, `@sha256:[0-9a-f]{64}$`, reuseImage)

	darwin := Step("/src/repo", "/src/repo/.git", "darwin", env)
	assert.Equal(t, "not run on darwin: reuse runs from a Linux container image; the hosted Linux runners run it", darwin.Skip)
	assert.Empty(t, darwin.Argv, "nothing is started")

	require.Error(t, Step("/src/a,b", "/src/a,b/.git", "linux", env).Unavailable, "a path the mount option cannot name")
	require.Error(t, Step("/src/a", "/src/b=c/.git", "linux", env).Unavailable, "a git directory the mount option cannot name")

	var out bytes.Buffer
	c := check.NewPrinter(&out)
	c.Run(t.Context(), t.TempDir(), []check.Step{citest.Passing, darwin})
	assert.True(t, c.OK, "a step not run fails nothing")
	assert.Contains(t, out.String(), "--    license: "+darwin.Skip+"\n")
	assert.NotContains(t, out.String(), "ok    license")
	assert.NotContains(t, out.String(), "FAIL  license")
}
