// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
// script: go, and through its mise shim mise, start with miseEnv.
var hookLine = "#!/bin/sh\nexec env " + strings.Join(miseEnv, " ") + " go run ./tools/ci fast \"$@\"\n"

// fixtureRecord is the one decision record of a fixture.
const fixtureRecord = "# 1. Pick a thing\n\nDate: 2026-01-01\n\n## Status\n\nAccepted\n"

// newTree returns a fixture repository that passes hygiene and
// generated: the hook, the two data files, a denylist with the made-up
// name, one page, the sources of the generators (go.mod, the ask-first
// list, one record) and the files they write. Nothing is committed yet.
func newTree(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	r.Write(".githooks/pre-push", hookLine)
	require.NoError(t, os.Chmod(filepath.Join(r.Dir, ".githooks", "pre-push"), 0o744))
	r.Write("tools/ci/prose.yaml", fixtureProse)
	r.Write(names.Path, string(denylist(t, madeUp)))
	r.Write("README.md", "# A fixture\n\nPlain text.\n")
	r.Write("go.mod", "module "+fixtureModule+"\n\ngo 1.27.0\n")
	r.Write(askFirstPath, fixtureList)
	r.Write("docs/adr/0001-pick-a-thing.md", fixtureRecord)
	files, err := generateFiles(os.DirFS(r.Dir))
	require.NoError(t, err)
	for _, f := range files {
		r.Write(f.path, string(f.data))
	}
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
	// The ceiling keeps git from finding a repository above the
	// temporary directory, when TMPDIR is inside a working tree.
	r.Env = append(gittest.Env(t), "GIT_CEILING_DIRECTORIES="+filepath.Dir(r.Dir))
	for _, args := range [][]string{{"hygiene"}, {"fast"}} {
		code, out := runCI(t, r, nil, nil, args...)
		assert.Equal(t, exitError, code, "%v: exit status\n%s", args, out)
	}
}

// TestHook checks the tracked hook against the one line of 12 12.1,
// runs it with a fake go that prints what it gets (the variables of
// miseEnv, set or not, and its arguments), and checks the mode git
// records for it.
func TestHook(t *testing.T) {
	path := filepath.Join("..", "..", ".githooks", "pre-push")
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, hookLine, string(got), "the hook")

	envPath, err := exec.LookPath("env")
	require.NoError(t, err)
	bin := t.TempDir()
	// The hook runs with each variable of miseEnv set to another
	// value, and go gets the one of miseEnv.
	environ := []string{"PATH=" + bin + string(os.PathListSeparator) + filepath.Dir(envPath)}
	var printed []string
	for _, kv := range miseEnv {
		key, _, _ := strings.Cut(kv, "=")
		printed = append(printed, key+"=${"+key+"-unset}")
		environ = append(environ, key+"=from-the-process")
	}
	fake := "#!/bin/sh\necho \"" + strings.Join(printed, " ") + " $*\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "go"), []byte(fake), 0o700))
	cmd := exec.CommandContext(t.Context(), "/bin/sh", path, "origin", "https://example.invalid/r.git")
	cmd.Env = environ
	ran, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, strings.Join(miseEnv, " ")+" run ./tools/ci fast origin https://example.invalid/r.git\n", string(ran), "what go gets from the hook")

	// The index is read with this process's environment: inside the
	// hook, git names the repository through it.
	out, err := exec.Command("git", "ls-files", "--stage", "--", path).Output()
	if err != nil {
		t.Skipf("the mode is not checked: this copy of the source is not a git working tree (%v)", err)
	}
	mode, _, _ := strings.Cut(string(out), " ")
	assert.Equal(t, hookMode, mode, "the mode git records for the hook")
}

// TestPullRequestTemplate checks the template's level-2 headings
// against the five sections of 12 12.7, in their order, so that a
// pull request starts with the sections the pr check reads.
func TestPullRequestTemplate(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(moduleRoot(t), ".github", "pull_request_template.md"))
	require.NoError(t, err)
	var headings []string
	for line := range strings.Lines(string(data)) {
		if strings.HasPrefix(line, "## ") {
			headings = append(headings, strings.TrimSpace(line))
		}
	}
	assert.Equal(t, []string{"## Why", "## What changed", "## Evidence", "## Middleware", "## Lessons"}, headings)
}
