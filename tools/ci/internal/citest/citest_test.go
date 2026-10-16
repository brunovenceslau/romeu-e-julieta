// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package citest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
)

// TestNewTree checks that the fixture holds the tracked hook, executable,
// the word lists and a denylist with the made-up name.
func TestNewTree(t *testing.T) {
	r := NewTree(t)
	hook, err := os.ReadFile(filepath.Join(ModuleRoot(t), ".githooks", "pre-push"))
	require.NoError(t, err)
	info, err := os.Stat(filepath.Join(r.Dir, ".githooks", "pre-push"))
	require.NoError(t, err)
	assert.NotZero(t, info.Mode().Perm()&0o100, "the hook is executable")
	got, err := os.ReadFile(filepath.Join(r.Dir, ".githooks", "pre-push"))
	require.NoError(t, err)
	assert.Equal(t, string(hook), string(got))
	data, err := os.ReadFile(filepath.Join(r.Dir, names.Path))
	require.NoError(t, err)
	list, err := names.Parse(data)
	require.NoError(t, err)
	assert.True(t, list.Match([]byte("a "+MadeUp+" here")))
}

// TestModuleRoot checks that the root holds go.mod and this package.
func TestModuleRoot(t *testing.T) {
	root := ModuleRoot(t)
	assert.FileExists(t, filepath.Join(root, "go.mod"))
	assert.DirExists(t, filepath.Join(root, "tools", "ci", "internal", "citest"))
	assert.Equal(t, root, MustAbs(t, root))
	assert.Equal(t, MustEval(t, root), MustEval(t, MustEval(t, root)))
}

// TestProfileOf checks the profile of two packages.
func TestProfileOf(t *testing.T) {
	assert.Equal(t, "mode: atomic\n"+
		FixtureModule+"/a/f.go:1.2,3.4 1 1\n"+
		FixtureModule+"/b/f.go:11.2,3.4 2 0\n", string(ProfileOf("a 1 1", "b 2 0")))
}

// TestEdits checks that an edit replaces its text once.
func TestEdits(t *testing.T) {
	got := Edits(t, "name: ci\n", "name: other\n")
	assert.True(t, strings.HasPrefix(got, "name: other\n"))
	assert.Contains(t, got, WorkflowEnv)
	assert.Contains(t, got, Sha)
}

// TestFakeMise checks the home, the installs directory, a tool and the
// script of the fake mise, and the lock of the fakes.
func TestFakeMise(t *testing.T) {
	IsolateMiseConfig(t)
	f := NewFakeMise(t)
	assert.Equal(t, f.Home, os.Getenv("HOME"))
	assert.Equal(t, f.Bin, os.Getenv("PATH"))
	assert.DirExists(t, f.Installs())
	assert.FileExists(t, f.Tool(t, "go", "1.27.0", "bin", "go"))
	f.Script(t, "", "/lint", "/go")
	script, err := os.ReadFile(filepath.Join(f.Bin, "mise"))
	require.NoError(t, err)
	assert.Contains(t, string(script), "golangci-lint) echo '/lint'")
	lock, err := os.ReadFile(filepath.Join(LockedRoot(t, LockFixture), "mise.lock"))
	require.NoError(t, err)
	assert.Equal(t, LockFixture, string(lock))
	assert.Equal(t, "passes", Passing.Name)
	assert.Equal(t, "fails", Failing.Name)
	assert.NotEmpty(t, Denylist(t))
	assert.Contains(t, FixtureProse, "glimmerous")
}
