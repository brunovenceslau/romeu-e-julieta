// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
)

// madeUp is the forbidden name of every fixture here: an invented word
// pair that names nothing.
const madeUp = "zorvex quimby"

// fixtureProse is a made-up word list, so that no test file holds a
// word the repository really bans.
const fixtureProse = `rules:
  - id: shiny
    phrases: [glimmerous]
    fails: [a glimmerous tool]
    passes: [a plain tool]
`

// hookLine is the hook of 12 12.1, with the line that makes it a
// script: it starts go, and so the mise shim, with goBuildEnv and miseEnv
// (tools/ci/misefiles.go), each variable a word of env.
var hookLine = "#!/bin/sh\nexec env " + strings.Join(append(slices.Clone(goBuildEnv), miseEnv...), " ") + " go run ./tools/ci fast \"$@\"\n"

// newTree returns a fixture repository that passes hygiene: the hook,
// the two data files, a denylist with the made-up name, and one page.
// Nothing is committed yet.
func newTree(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	r.Write(".githooks/pre-push", hookLine)
	require.NoError(t, os.Chmod(filepath.Join(r.Dir, ".githooks", "pre-push"), 0o744))
	r.Write("tools/ci/prose.yaml", fixtureProse)
	r.Write(names.Path, string(denylist(t, madeUp)))
	r.Write("README.md", "# A fixture\n\nPlain text.\n")
	return r
}

func denylist(t *testing.T, madeUpNames ...string) []byte {
	t.Helper()
	l := &names.List{}
	for _, n := range madeUpNames {
		e, err := names.NewEntry(n)
		require.NoError(t, err)
		l.Add(e)
	}
	return l.Marshal()
}

// runCI runs the command in the fixture and returns its exit status
// and what it printed. The steps of fast are replaced by stubs: the
// real ones run the tests this file is part of.
func runCI(t *testing.T, r *gittest.Repo, stdin io.Reader, steps []step, args ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	e := env{dir: r.Dir, gitEnv: r.Env, stdin: stdin, stdout: &out, stderr: &out, steps: steps}
	return run(t.Context(), e, args), out.String()
}

func TestUsage(t *testing.T) {
	r := newTree(t)
	r.Commit("fixture")
	tests := []struct {
		name string
		args []string
	}{
		{"no subcommand", nil},
		{"an unknown subcommand", []string{"nonesuch"}},
		{"hygiene with a stray argument", []string{"hygiene", "stray"}},
		{"hygiene with an unknown flag", []string{"hygiene", "--nonesuch"}},
		{"fast with one argument", []string{"fast", "origin"}},
		{"fast with three arguments", []string{"fast", "origin", "url", "more"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, out := runCI(t, r, nil, nil, tt.args...)
			assert.Equal(t, exitError, code, "exit status\n%s", out)
		})
	}
}

func TestOutsideARepository(t *testing.T) {
	r := &gittest.Repo{}
	r.Dir = t.TempDir()
	// Env's ceiling keeps git from finding a repository above the
	// temporary directory, when TMPDIR is inside a working tree.
	r.Env = gittest.Env(t)
	for _, args := range [][]string{{"hygiene"}, {"fast"}} {
		code, out := runCI(t, r, nil, nil, args...)
		assert.Equal(t, exitError, code, "%v: exit status\n%s", args, out)
	}
}

// TestHook checks the tracked hook against the one line of 12 12.1,
// and the mode git records for it.
func TestHook(t *testing.T) {
	path := filepath.Join("..", "..", ".githooks", "pre-push")
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, hookLine, string(got), "the hook")
	// The index is read with this process's environment: inside the
	// hook, git names the repository through it.
	out, err := exec.Command("git", "ls-files", "--stage", "--", path).Output()
	if err != nil {
		t.Skipf("the mode is not checked: this copy of the source is not a git working tree (%v)", err)
	}
	mode, _, _ := strings.Cut(string(out), " ")
	assert.Equal(t, hookMode, mode, "the mode git records for the hook")
}

// TestRunPrintsAnErrorAsSafeText checks that the error line of run does
// not carry the control characters or the command line of an argument.
func TestRunPrintsAnErrorAsSafeText(t *testing.T) {
	var out, errOut bytes.Buffer
	e := env{dir: t.TempDir(), stdout: &out, stderr: &errOut}
	assert.Equal(t, exitError, run(context.Background(), e, []string{"hygiene", "--bad\x1b[2J\n::error::forged"}))
	assert.NotContains(t, errOut.String(), "\x1b")
	for _, line := range strings.Split(errOut.String(), "\n") {
		assert.False(t, strings.HasPrefix(strings.TrimSpace(line), "::"), "%q", line)
	}
	assert.Contains(t, errOut.String(), `\x1b[2J`)
}

// TestGoBuildEnv pins goBuildEnv, and that stepEnv holds it. The hook
// holds it through hookLine, which TestHook matches byte for byte.
func TestGoBuildEnv(t *testing.T) {
	assert.Equal(t, []string{"GOWORK=off", "GOFLAGS=-mod=readonly"}, goBuildEnv)
	assert.Subset(t, stepEnv("/go"), goBuildEnv, "stepEnv holds goBuildEnv")
}

// TestHookStartsGoWithoutTheCallersGoEnv runs the tracked hook with a
// go env file, a go work file, a GOFLAGS and a GOTOOLCHAIN in the
// caller's environment, and a go stub that prints what it was started
// with: all four are replaced, and the arguments of git pass through.
func TestHookStartsGoWithoutTheCallersGoEnv(t *testing.T) {
	bin := t.TempDir()
	stub := "#!/bin/sh\nprintf 'GOENV=%s GOWORK=%s GOFLAGS=%s GOTOOLCHAIN=%s args=' \"${GOENV-unset}\" \"${GOWORK-unset}\" \"${GOFLAGS-unset}\" \"${GOTOOLCHAIN-unset}\"; for a; do printf '[%s]' \"$a\"; done; echo\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "go"), []byte(stub), 0o700))
	cmd := exec.Command("sh", filepath.Join("..", "..", ".githooks", "pre-push"), "origin", "https://example.test/a b.git")
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GOENV=/elsewhere/env", "GOWORK=/elsewhere/go.work", "GOFLAGS=-overlay=/tmp/x.json", "GOTOOLCHAIN=go9.9")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Equal(t, "GOENV=off GOWORK=off GOFLAGS=-mod=readonly GOTOOLCHAIN=local args=[run][./tools/ci][fast][origin][https://example.test/a b.git]\n", string(out))
}
