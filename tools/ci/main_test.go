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
// script: it starts go, and so the mise shim, in an environment built
// from nothing ("env -i"): the variables of hookAllow that the caller
// has set, each as ${K:+K="$K"} so an unset one stays unset and a value
// with a space stays one word; then goBuildEnv, miseEnv and the ceiling
// of the mise files (the parent of the tree, which is the working
// directory of a hook), each variable a word of env.
var hookLine = "#!/bin/sh\nexec env -i " + hookAllowWords() + " " +
	strings.Join(slices.Concat(goBuildEnv, miseEnv), " ") +
	` MISE_CEILING_PATHS="$(dirname -- "$PWD")" go run ./tools/ci fast "$@"` + "\n"

func hookAllowWords() string {
	words := make([]string, len(hookAllow))
	for i, k := range hookAllow {
		words[i] = "${" + k + `:+` + k + `="$` + k + `"}`
	}
	return strings.Join(words, " ")
}

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

// TestHookEnvIsTheAllowlist runs the tracked hook with a hostile
// environment, every variable of the escapes the audits found (a go env
// file, a go work file, GOFLAGS with -overlay, a GOTOOLCHAIN, a mise
// global and system configuration, MISE_ENV_FILE, MISE_CD,
// MISE_TRUSTED_CONFIG_PATHS, MISE_CONFIG_DIR, GIT_DIR, and a MISE_FOO
// and a GOFOO that no list names) besides the variables of the
// allowlist, one of them with a space, and a go stub that prints what it
// was started with. The stub gets exactly the variables of the
// allowlist that were set, the fixed settings and the ceiling, the
// arguments of git pass through as they are, and nothing else reaches
// it.
func TestHookEnvIsTheAllowlist(t *testing.T) {
	bin, tree := t.TempDir(), t.TempDir()
	stub := "#!/bin/sh\nenv\nfor a; do printf 'ARG[%s]\\n' \"$a\"; done\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "go"), []byte(stub), 0o700))
	allowed := map[string]string{
		"PATH": bin + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME": "/home/with a space", "TMPDIR": "/tmp/x",
		"GOCACHE": "/cache/go", "HTTPS_PROXY": "http://proxy.invalid", "MISE_STATE_DIR": "/state",
	}
	hostile := map[string]string{
		"GOENV": "/elsewhere/env", "GOWORK": "/elsewhere/go.work", "GOFLAGS": "-overlay=/tmp/x.json",
		"GOTOOLCHAIN": "go9.9", "GOPROXY": "https://evil.invalid", "GOEXPERIMENT": "x", "GODEBUG": "x", "CGO_ENABLED": "1",
		"GOFOO": "1", "MISE_FOO": "1", "MISE_ENV_FILE": ".env", "MISE_CD": "/elsewhere", "MISE_TRUSTED_CONFIG_PATHS": "/",
		"MISE_CONFIG_DIR": "/elsewhere/cfg", "MISE_GLOBAL_CONFIG_FILE": "/elsewhere/g.toml",
		"MISE_SYSTEM_CONFIG_FILE": "/elsewhere/s.toml", "MISE_CEILING_PATHS": "/", "MISE_OVERRIDE_CONFIG_FILENAMES": "x.toml",
		"GIT_DIR": "/elsewhere/.git", "BASH_ENV": "/elsewhere/rc",
	}
	cmd := exec.Command("sh", mustAbs(t, filepath.Join("..", "..", ".githooks", "pre-push")), "origin", "https://example.test/a b.git")
	cmd.Dir = tree
	for _, m := range []map[string]string{allowed, hostile} {
		for k, v := range m {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	got := map[string]string{}
	var args []string
	for line := range strings.Lines(string(out)) {
		line = strings.TrimSuffix(line, "\n")
		if strings.HasPrefix(line, "ARG[") {
			args = append(args, strings.TrimSuffix(strings.TrimPrefix(line, "ARG["), "]"))
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		if !slices.Contains([]string{"PWD", "OLDPWD", "SHLVL", "_"}, k) {
			got[k] = v
		}
	}
	want := map[string]string{}
	for k, v := range allowed {
		want[k] = v
	}
	for _, kv := range slices.Concat(goBuildEnv, miseEnv) {
		k, v, _ := strings.Cut(kv, "=")
		want[k] = v
	}
	realTree, err := filepath.EvalSymlinks(tree)
	require.NoError(t, err)
	assert.Equal(t, filepath.Dir(realTree), mustEval(t, got["MISE_CEILING_PATHS"]), "the ceiling is the parent of the working directory")
	want["MISE_CEILING_PATHS"] = got["MISE_CEILING_PATHS"]
	assert.Equal(t, want, got, "the variables go gets")
	assert.Equal(t, []string{"run", "./tools/ci", "fast", "origin", "https://example.test/a b.git"}, args)
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	require.NoError(t, err)
	return abs
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(p)
	require.NoError(t, err)
	return real
}
