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
// script.
const hookLine = "#!/bin/sh\nexec go run ./tools/ci fast \"$@\"\n"

// newTree returns a fixture repository that passes hygiene: the hook,
// the two data files, a denylist with the made-up name, and one page.
// Nothing is committed yet.
func newTree(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	r.Write(".githooks/pre-push", hookLine)
	if err := os.Chmod(filepath.Join(r.Dir, ".githooks", "pre-push"), 0o744); err != nil {
		t.Fatal(err)
	}
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
		if err != nil {
			t.Fatal(err)
		}
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
			if code, out := runCI(t, r, nil, nil, tt.args...); code != exitError {
				t.Errorf("exit = %d, want %d\n%s", code, exitError, out)
			}
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
		if code, out := runCI(t, r, nil, nil, args...); code != exitError {
			t.Errorf("%v: exit = %d, want %d\n%s", args, code, exitError, out)
		}
	}
}

// TestHook checks the tracked hook against the one line of 12 12.1,
// and the mode git records for it.
func TestHook(t *testing.T) {
	path := filepath.Join("..", "..", ".githooks", "pre-push")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != hookLine {
		t.Errorf("the hook is %q, want %q", got, hookLine)
	}
	// The index is read with this process's environment: inside the
	// hook, git names the repository through it.
	out, err := exec.Command("git", "ls-files", "--stage", "--", path).Output()
	if err != nil {
		t.Skipf("the mode is not checked: this copy of the source is not a git working tree (%v)", err)
	}
	if mode, _, _ := strings.Cut(string(out), " "); mode != hookMode {
		t.Errorf("git records the mode %q for the hook, want %s", mode, hookMode)
	}
}
