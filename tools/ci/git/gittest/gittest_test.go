// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package gittest_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// TestFixturesAreReproducible checks the promise of Env: the same steps
// give the same commit ids, on any machine and in any directory.
func TestFixturesAreReproducible(t *testing.T) {
	build := func() string {
		r := gittest.New(t)
		r.Write("docs/a.md", "a\n")
		return r.Commit("first")
	}
	first := build()
	assert.Regexp(t, `^[0-9a-f]{40}$`, first)
	assert.Equal(t, first, build())
}

// TestRepo covers the helpers a fixture is made with: a file written
// with its directories, an empty commit, another identity, a raw
// object, a bare repository to push to, and a repository bound to a
// subtest.
func TestRepo(t *testing.T) {
	r := gittest.New(t)
	assert.Equal(t, "main", r.Git("symbolic-ref", "--short", "HEAD"))

	r.Write("a/b/c.txt", "c\n")
	content, err := os.ReadFile(filepath.Join(r.Dir, "a", "b", "c.txt"))
	require.NoError(t, err)
	assert.Equal(t, "c\n", string(content))
	one := r.Commit("one")
	two := r.Commit("an empty commit")
	assert.NotEqual(t, one, two)
	assert.Equal(t, one, r.Git("rev-parse", "HEAD~1"))

	other := r.WithEnv("GIT_AUTHOR_NAME=Another Author")
	other.Commit("by another")
	assert.Equal(t, "Another Author", r.Git("log", "-1", "--format=%an"))
	assert.Equal(t, "Test Author", r.Git("log", "-1", "--format=%an", "HEAD~1"), "WithEnv leaves the first repository's identity alone")

	blob := r.WriteObject("blob", "raw\n")
	assert.Equal(t, "raw", r.Git("cat-file", "-p", blob))

	bare := gittest.NewBare(t)
	assert.Equal(t, "true", bare.Git("rev-parse", "--is-bare-repository"))
	r.Git("push", "--quiet", bare.Dir, "main")
	assert.Equal(t, r.Git("rev-parse", "HEAD"), bare.Git("rev-parse", "main"))

	t.Run("bound to a subtest", func(t *testing.T) {
		assert.Equal(t, r.Dir, r.For(t).Dir)
		assert.Equal(t, r.Git("rev-parse", "HEAD"), r.For(t).Git("rev-parse", "HEAD"))
	})
}

// TestEnv checks that a fixture reads no configuration of the machine.
func TestEnv(t *testing.T) {
	env := gittest.Env(t)
	assert.Contains(t, env, "GIT_CONFIG_GLOBAL="+os.DevNull)
	assert.Contains(t, env, "GIT_CONFIG_NOSYSTEM=1")
}

// TestEnvStopsGitAtTheTemporaryDirectories checks that a fixture that is
// not a repository is not one by accident of where TMPDIR lies: with the
// temporary directories inside a work tree, git run with Env finds no
// repository above them, while git run without the ceiling does.
func TestEnvStopsGitAtTheTemporaryDirectories(t *testing.T) {
	outer := gittest.New(t)
	t.Setenv("TMPDIR", outer.Dir) // before the first TempDir of the subtest
	t.Run("inside", func(t *testing.T) {
		env := gittest.Env(t)
		dir := t.TempDir()
		require.True(t, strings.HasPrefix(dir, outer.Dir), "the fixture directory lies in the outer work tree")
		inner := git.Repo{Dir: dir, Env: env}
		_, err := inner.Run(context.Background(), nil, "rev-parse", "--show-toplevel")
		require.Error(t, err, "the ceiling hides the outer repository")
		without := git.Repo{Dir: dir, Env: []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_GLOBAL=" + os.DevNull}}
		_, err = without.Run(context.Background(), nil, "rev-parse", "--show-toplevel")
		require.NoError(t, err, "without the ceiling git finds it, so the test above proves something")
	})
}
