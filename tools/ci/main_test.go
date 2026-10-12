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

// hookScript is the hook of 12 12.1, as a script. Every go it starts
// (run) is started in an environment built from nothing ("env -i"):
// the variables of hookAllow that the caller has set, each as
// ${K:+K="$K"} so an unset one stays unset and a value with a space
// stays one word; then goBuildEnv, miseEnv and the ceiling of the mise
// files (the parent of the tree, which is the working directory of a
// hook, with its symbolic links resolved: PWD is the path as the caller
// typed it), each variable a word of env. A probe, "go list -m", comes
// first: a go older than go.mod refuses it, and the hook says what to
// do, since the refusal is a bare line from a go that no Go code of the
// tree can improve.
var hookScript = "#!/bin/sh\n" +
	"# The environment of every go this hook starts, built from nothing.\n" +
	"run() {\n\tenv -i " + hookAllowWords() + " " +
	strings.Join(slices.Concat(goBuildEnv, miseEnv), " ") +
	` MISE_CEILING_PATHS="$(dirname -- "$(pwd -P)")" "$@"` + "\n}\n" +
	`# A go that is older than go.mod refuses with a bare "go: go.mod requires
# go >= ..." before tools/ci can say more, so this probe, which reads
# go.mod and builds nothing, lets go's own line through and says what to do.
# sh because no Go runs yet.
run go list -m >/dev/null </dev/null || {
	printf '%s\n' \
		"pre-push: the go on PATH ($(command -v go || echo none)) cannot run this module." \
		"pre-push: put the mise shims first on PATH, so that go is the pinned toolchain:" \
		"pre-push:   mise install, then mise activate in your shell profile (or mise activate --shims)." >&2
	exit 1
}
run go run ./tools/ci fast "$@"
`

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
	r.Write(".githooks/pre-push", hookScript)
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
	assert.Equal(t, hookScript, string(got), "the hook")
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
// MISE_TRUSTED_CONFIG_PATHS, MISE_CONFIG_DIR, XDG_CONFIG_HOME, GIT_DIR,
// and a MISE_FOO and a GOFOO that no list names) besides every variable
// of hookAllow, one of them with a space, and a go stub that prints what
// it was started with. The hook runs in a symbolic link to the tree,
// with PWD as the caller typed it. The stub gets exactly what the
// production lists build for a mise run at the top of the tree
// (miseRunEnviron over passThroughEnv), the proxy and module variables
// that were set, and goBuildEnv, with the ceiling the parent of the
// resolved tree; and the arguments of git pass through as they are.
func TestHookEnvIsTheAllowlist(t *testing.T) {
	bin, real := t.TempDir(), mustEval(t, t.TempDir())
	linkDir := t.TempDir()
	link := filepath.Join(linkDir, "tree")
	require.NoError(t, os.Symlink(real, link))
	stub := "#!/bin/sh\n[ \"$1\" = run ] || exit 0\nenv\nfor a; do printf 'ARG[%s]\\n' \"$a\"; done\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "go"), []byte(stub), 0o700))
	path := bin + string(os.PathListSeparator) + os.Getenv("PATH")
	for _, key := range hookAllow {
		t.Setenv(key, "/allowed/"+key)
	}
	t.Setenv("PATH", path)
	t.Setenv("HOME", "/home/with a space")
	for k, v := range map[string]string{
		"GOENV": "/elsewhere/env", "GOWORK": "/elsewhere/go.work", "GOFLAGS": "-overlay=/tmp/x.json",
		"GOTOOLCHAIN": "go9.9", "GOEXPERIMENT": "x", "GODEBUG": "x", "CGO_ENABLED": "1",
		"GOFOO": "1", "MISE_FOO": "1", "MISE_ENV_FILE": ".env", "MISE_CD": "/elsewhere", "MISE_TRUSTED_CONFIG_PATHS": "/",
		"MISE_CONFIG_DIR": "/elsewhere/cfg", "MISE_GLOBAL_CONFIG_FILE": "/elsewhere/g.toml",
		"MISE_SYSTEM_CONFIG_FILE": "/elsewhere/s.toml", "MISE_CEILING_PATHS": "/", "MISE_OVERRIDE_CONFIG_FILENAMES": "x.toml",
		"XDG_CONFIG_HOME": "/elsewhere/xdg", "GIT_DIR": "/elsewhere/.git", "BASH_ENV": "/elsewhere/rc",
	} {
		t.Setenv(k, v)
	}
	// What the production lists build, from the same process environment.
	want := map[string]string{}
	for _, kv := range slices.Concat(
		miseRunEnviron(withProcessEnv(passThroughEnv(), slices.Concat(networkPassThrough, goModulePassThrough)), real),
		goBuildEnv,
	) {
		k, v, _ := strings.Cut(kv, "=")
		want[k] = v
	}
	cmd := exec.Command("sh", mustAbs(t, filepath.Join("..", "..", ".githooks", "pre-push")), "origin", "https://example.test/a b.git")
	cmd.Dir = link
	cmd.Env = append(os.Environ(), "PWD="+link) // the path as the caller typed it
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
	assert.Equal(t, "MISE_CEILING_PATHS="+filepath.Dir(real), "MISE_CEILING_PATHS="+got["MISE_CEILING_PATHS"],
		"the ceiling is the parent of the resolved tree, not of the path typed")
	assert.NotEqual(t, filepath.Dir(link), got["MISE_CEILING_PATHS"])
	assert.Equal(t, want, got, "the variables go gets")
	assert.Equal(t, []string{"run", "./tools/ci", "fast", "origin", "https://example.test/a b.git"}, args)
}

// TestHookExplainsAToolchainRefusal runs the tracked hook with a go
// stub that refuses the probe the way a go older than go.mod does: the
// hook lets the refusal through, adds what to do, exits non-zero and
// never starts the checks. With a go that accepts the probe the hook
// starts the checks, with the arguments of git, and its exit status is
// theirs; the probe does not read the refs git sends on the standard
// input. The stub is a shell script, which the four runners of CI run.
func TestHookExplainsAToolchainRefusal(t *testing.T) {
	const refusal = "go: go.mod requires go >= 9.9.9 (running go 1.0.0; GOTOOLCHAIN=local)"
	hook := mustAbs(t, filepath.Join("..", "..", ".githooks", "pre-push"))
	// The stub refuses "list" when REFUSE is set, which the hook does not
	// pass on to it (env -i): the file next to the stub says it.
	stub := "#!/bin/sh\n" +
		"here=$(dirname -- \"$0\")\n" +
		"if [ \"$1\" = list ]; then\n" +
		"\t[ -e \"$here/refuse\" ] || exit 0\n" +
		"\techo '" + refusal + "' >&2\n\texit 1\nfi\n" +
		"echo \"checks $*\"\ncat >/dev/null\nexit 7\n"
	tests := []struct {
		name       string
		refuse     bool
		wantCode   int
		wantOut    []string
		wantAbsent []string
	}{
		{"a go older than go.mod", true, 1,
			[]string{refusal, "cannot run this module", "put the mise shims first on PATH", "mise activate"},
			[]string{"checks"}},
		{"a go that accepts the probe", false, 7,
			[]string{"checks run ./tools/ci fast origin https://example.test/a b.git"},
			[]string{"cannot run this module", refusal}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bin := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(bin, "go"), []byte(stub), 0o700))
			if tt.refuse {
				require.NoError(t, os.WriteFile(filepath.Join(bin, "refuse"), nil, 0o600))
			}
			cmd := exec.Command("sh", hook, "origin", "https://example.test/a b.git")
			cmd.Dir = t.TempDir()
			cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			cmd.Stdin = strings.NewReader("refs/heads/x 0 refs/heads/x 0\n")
			out, err := cmd.CombinedOutput()
			code := 0
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantCode, code, "exit status\n%s", out)
			for _, w := range tt.wantOut {
				assert.Contains(t, string(out), w)
			}
			for _, w := range tt.wantAbsent {
				assert.NotContains(t, string(out), w)
			}
			if tt.refuse {
				assert.Contains(t, string(out), filepath.Join(bin, "go"), "the hint names the go it found")
			}
		})
	}
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
