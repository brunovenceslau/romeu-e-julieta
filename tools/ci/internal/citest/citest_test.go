// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package citest

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
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

// TestRun checks the three statuses of a subcommand and that the error
// is printed.
func TestRun(t *testing.T) {
	r := NewTree(t)
	tests := []struct {
		ok   bool
		err  error
		code int
		out  string
	}{
		{true, nil, check.ExitOK, ""},
		{false, nil, check.ExitFail, ""},
		{false, errors.New("broken"), check.ExitError, "ci: broken\n"},
	}
	for _, tt := range tests {
		code, out := Run(t, r, nil, func(_ context.Context, e check.Env, args []string) (bool, error) {
			assert.Equal(t, []string{"x"}, args)
			assert.Equal(t, r.Dir, e.Dir)
			return tt.ok, tt.err
		}, "x")
		assert.Equal(t, tt.code, code)
		assert.Equal(t, tt.out, out)
	}
}

// TestMiseHelpers checks the environment of a mise built from nothing,
// the workflow that pins a mise version, and the tools mise resolves
// after a trust, with a fake mise and a resolver of the test.
func TestMiseHelpers(t *testing.T) {
	env := IsolatedMiseEnv(t)
	assert.Contains(t, env, "MISE_OFFLINE=1")
	assert.Contains(t, MiseWorkflow(t, FakeMiseVersion), "version: "+FakeMiseVersion+"\n")
	assert.Contains(t, AnswerVersion, FakeMiseVersion)

	f := NewFakeMise(t)
	require.NoError(t, os.WriteFile(filepath.Join(f.Bin, "mise"), []byte("#!/bin/sh\nexit 0\n"), 0o700))
	var trusted []string
	want := check.Tools{Linter: "/l", GoDir: "/g"}
	got := PinnedLintTools(t, func(env []string) []string {
		trusted = env
		return append(env, "PATH="+f.Bin)
	}, func(_ context.Context, root string) (check.Tools, error) {
		assert.Equal(t, ModuleRoot(t), root)
		return want, nil
	})
	assert.Equal(t, want, got)
	assert.NotNil(t, trusted, "the trust runs in the environment of a mise run")
}

// TestGitHelpers checks the partial clone, which holds no blob, and the
// root of a directory.
func TestGitHelpers(t *testing.T) {
	upstream := NewTree(t)
	upstream.Commit("fixture")
	clone := PartialClone(t, upstream)
	out, err := clone.Run(t.Context(), nil, "rev-parse", "HEAD")
	require.NoError(t, err)
	assert.Equal(t, upstream.Git("rev-parse", "HEAD"), strings.TrimSpace(string(out)))
	root := RootOf(t, t.TempDir())
	require.NoError(t, root.WriteFile("f", []byte("x"), 0o600))
}

// TestRangeOfAndExit checks how verdicts read back, and the exit status
// of a subcommand run with an environment of the test's own.
func TestRangeOfAndExit(t *testing.T) {
	f := check.Finding{Where: "a", Rule: "r", Msg: "m"}
	findings, checked, err := RangeOf([]check.Verdict{{Findings: []check.Finding{f}}}, nil)
	require.NoError(t, err)
	assert.True(t, checked)
	assert.Equal(t, []check.Finding{f}, findings)
	_, checked, err = RangeOf([]check.Verdict{{Note: "not checked"}}, nil)
	require.NoError(t, err)
	assert.False(t, checked)
	_, _, err = RangeOf([]check.Verdict{{Err: errors.New("broken")}}, nil)
	require.EqualError(t, err, "broken")

	var out bytes.Buffer
	e := check.Env{Stdout: &out, Stderr: &out}
	assert.Equal(t, check.ExitError, Exit(t.Context(), e, func(context.Context, check.Env, []string) (bool, error) {
		return false, errors.New("broken")
	}, "x"))
	assert.Equal(t, "ci: broken\n", out.String())
}
