// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/goenv"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/hygiene"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
)

// hookScript is the tracked hook, with the words of its env filled in
// from the lists of the code. Every go the hook starts (run) is started
// with "exec env -i" in an environment built from nothing: the
// variables of hookAllow that the caller has set, each as
// ${K:+K="$K"} so an unset one stays unset and a value with a space
// stays one word; then goenv.Build, miseEnv and the ceiling of the mise
// files (the parent of the tree, which is the working directory of a
// hook, with its symbolic links resolved: PWD is the path as the caller
// typed it), each variable a word of env. The exec makes a signal sent
// to the hook reach go. A probe, "go list -m", comes first: a go older
// than go.mod refuses it, and the hook says what to do, since the
// refusal is a bare line from a go that no Go code of the tree can
// improve.
var hookScript = strings.Replace(hookTemplate, "{{ENV}}",
	hookAllowWords()+" "+strings.Join(slices.Concat(goenv.Build, mise.MiseEnv), " ")+
		` MISE_CEILING_PATHS="$(dirname -- "$(pwd -P)")"`, 1)

const hookTemplate = `#!/bin/sh
# The environment of every go this hook starts, built from nothing. It
# execs, so that a signal sent to the hook reaches go.
run() {
	exec env -i {{ENV}} "$@"
}
# A go older than go.mod refuses with a bare "go: go.mod requires go >=
# ..." before tools/ci can run, so this probe, which reads go.mod and
# builds nothing, lets go's own line through and then says what to do.
# It is shell because it runs before any Go code can.
gopath=$(command -v go) || gopath=
if [ -z "$gopath" ] || ! (run go list -m) >/dev/null </dev/null; then
	{
		case $gopath in
		'')
			printf '%s\n' \
				"pre-push: no go is on PATH." \
				"pre-push: install mise, then put its shims on PATH, in your shell profile (bash and zsh):" \
				'pre-push:   eval "$(mise activate <shell> --shims)"' \
				"pre-push: for another shell, see mise activate --help."
			;;
		*/mise/shims/*)
			printf '%s\n' \
				"pre-push: the go on PATH ($gopath) is a mise shim that cannot run this module." \
				"pre-push: run \"mise trust\" and \"mise install\" in $(pwd -P), then push again."
			;;
		*)
			printf '%s\n' \
				"pre-push: the go on PATH ($gopath) cannot run this module." \
				"pre-push: put the mise shims first on PATH, so that go is the pinned toolchain, in your shell profile (bash and zsh):" \
				'pre-push:   eval "$(mise activate <shell> --shims)"' \
				"pre-push: for another shell, see mise activate --help."
			;;
		esac
	} >&2
	exit 1
fi
run go run ./tools/ci fast "$@"
`

func hookAllowWords() string {
	words := make([]string, len(mise.HookAllow))
	for i, k := range mise.HookAllow {
		words[i] = "${" + k + `:+` + k + `="$` + k + `"}`
	}
	return strings.Join(words, " ")
}

// runCI runs the command in the fixture and returns its exit status
// and what it printed. The steps of fast are replaced by stubs: the
// real ones run the tests this file is part of.
func runCI(t *testing.T, r *gittest.Repo, stdin io.Reader, steps []check.Step, args ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	e := check.Env{Dir: r.Dir, GitEnv: r.Env, Stdin: stdin, Stdout: &out, Stderr: &out, Steps: steps}
	return run(t.Context(), e, args), out.String()
}

func TestUsage(t *testing.T) {
	r := citest.NewTree(t)
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
			assert.Equal(t, check.ExitError, code, "exit status\n%s", out)
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
		assert.Equal(t, check.ExitError, code, "%v: exit status\n%s", args, out)
	}
}

// TestHook checks the tracked hook against hookScript, and the mode
// git records for it.
func TestHook(t *testing.T) {
	path := filepath.Join("..", "..", ".githooks", "pre-push")
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, hookScript, string(got), "the hook")
	// The index is read with this process's environment: inside the
	// hook, git names the repository through it.
	var out strings.Builder
	err = procgroup.Run(t.Context(), procgroup.Command{
		Class:  procgroup.ToolGit,
		Argv:   []string{"git", "ls-files", "--stage", "--", path},
		Env:    os.Environ(),
		Stdout: &out,
	})
	if err != nil {
		t.Skipf("the mode is not checked: this copy of the source is not a git working tree (%v)", err)
	}
	mode, _, _ := strings.Cut(out.String(), " ")
	assert.Equal(t, hygiene.HookMode, mode, "the mode git records for the hook")
}

// TestRunPrintsAnErrorAsSafeText checks that the error line of run does
// not carry the control characters or the command line of an argument.
func TestRunPrintsAnErrorAsSafeText(t *testing.T) {
	var out, errOut bytes.Buffer
	e := check.Env{Dir: t.TempDir(), Stdout: &out, Stderr: &errOut}
	assert.Equal(t, check.ExitError, run(context.Background(), e, []string{"hygiene", "--bad\x1b[2J\n::error::forged"}))
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
// each call (the probe and the checks) was started with. The hook runs
// in a symbolic link to the tree, with PWD as the caller typed it. Each call gets exactly what the
// production lists build for a mise run at the top of the tree
// (miseRunEnviron over passThroughEnv), the proxy and module variables
// that were set, and goenv.Build, with the ceiling the parent of the
// resolved tree; and the arguments of git pass through as they are.
func TestHookEnvIsTheAllowlist(t *testing.T) {
	bin, real := t.TempDir(), citest.MustEval(t, t.TempDir())
	linkDir := t.TempDir()
	link := filepath.Join(linkDir, "tree")
	require.NoError(t, os.Symlink(real, link))
	// The probe's standard output is discarded by the hook, so each call
	// appends to a file next to the stub.
	stub := "#!/bin/sh\n{\necho CALL\nenv\nfor a; do printf 'ARG[%s]\\n' \"$a\"; done\n} >> \"$(dirname -- \"$0\")/calls\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "go"), []byte(stub), 0o700))
	path := bin + string(os.PathListSeparator) + os.Getenv("PATH")
	for _, key := range mise.HookAllow {
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
		mise.RunEnviron(check.WithProcessEnv(check.PassThroughEnv(), slices.Concat(check.NetworkPassThrough, check.GoModulePassThrough)), real),
		goenv.Build,
	) {
		k, v, _ := strings.Cut(kv, "=")
		want[k] = v
	}
	hookOut, err := procgroup.CombinedOutput(t.Context(), procgroup.Command{
		Class: procgroup.ToolStep,
		Argv:  []string{"sh", citest.MustAbs(t, filepath.Join("..", "..", ".githooks", "pre-push")), "origin", "https://example.test/a b.git"},
		Dir:   link,
		Env:   append(os.Environ(), "PWD="+link), // the path as the caller typed it
	})
	require.NoError(t, err, string(hookOut))
	out, err := os.ReadFile(filepath.Join(bin, "calls"))
	require.NoError(t, err)
	// The stub prints one block per call: the probe, then the checks.
	var calls []map[string]string
	var argv [][]string
	for line := range strings.Lines(string(out)) {
		line = strings.TrimSuffix(line, "\n")
		if line == "CALL" {
			calls = append(calls, map[string]string{})
			argv = append(argv, nil)
			continue
		}
		last := len(calls) - 1
		if strings.HasPrefix(line, "ARG[") {
			argv[last] = append(argv[last], strings.TrimSuffix(strings.TrimPrefix(line, "ARG["), "]"))
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		if !slices.Contains([]string{"PWD", "OLDPWD", "SHLVL", "_"}, k) {
			calls[last][k] = v
		}
	}
	require.Len(t, calls, 2, "the probe and the checks")
	assert.Equal(t, []string{"list", "-m"}, argv[0], "the probe")
	assert.Equal(t, []string{"run", "./tools/ci", "fast", "origin", "https://example.test/a b.git"}, argv[1])
	for i, got := range calls {
		assert.Equal(t, "MISE_CEILING_PATHS="+filepath.Dir(real), "MISE_CEILING_PATHS="+got["MISE_CEILING_PATHS"],
			"the ceiling is the parent of the resolved tree, not of the path typed")
		assert.NotEqual(t, filepath.Dir(link), got["MISE_CEILING_PATHS"])
		assert.Equal(t, want, got, "the variables go gets, call %d", i)
	}
}

// hookStub is the go of the tests below. Its "list" call is the probe:
// it records its arguments, drains the standard input (so a probe that
// shared git's would eat the refs), prints a line on the standard
// output (which the hook discards) and refuses when "refuse" sits next
// to it, the way a go older than go.mod does. Any other call is the
// checks: it prints its arguments and the standard input it got, and
// exits 7; on SIGTERM it records it, and with "hang" next to it waits.
const hookStub = `#!/bin/sh
here=$(dirname -- "$0")
if [ "$1" = list ]; then
	echo "$*" > "$here/probe-argv"
	cat > /dev/null
	echo probe-stdout
	if [ -e "$here/refuse" ]; then
		echo 'go: go.mod requires go >= 9.9.9 (running go 1.0.0; GOTOOLCHAIN=local)' >&2
		exit 1
	fi
	exit 0
fi
trap 'echo term > "$here/term"; exit 143' TERM
echo "checks $*"
if [ -e "$here/hang" ]; then
	echo ready > "$here/ready"
	i=0
	while [ $i -lt 300 ]; do sleep 0.1; i=$((i+1)); done
fi
cat
exit 7
`

// runHook runs the tracked hook in a fresh directory with the given
// directory first on PATH (all of PATH when only is false, nothing else
// when it is true), and returns the exit code and the two streams.
func runHook(t *testing.T, bin string, only bool, stdin string) (code int, stdout, stderr string, dir string) {
	t.Helper()
	return runHookIn(t, citest.MustEval(t, t.TempDir()), "", bin, only, stdin)
}

// runHookIn is runHook in the directory dir; with link set, the hook
// runs in that symbolic link to dir, with PWD as the caller typed it.
func runHookIn(t *testing.T, dir, link, bin string, only bool, stdin string) (code int, stdout, stderr string, resolved string) {
	t.Helper()
	sh, err := procgroup.LookPath("sh")
	require.NoError(t, err)
	hook := citest.MustAbs(t, filepath.Join("..", "..", ".githooks", "pre-push"))
	path := bin
	if !only {
		path += string(os.PathListSeparator) + os.Getenv("PATH")
	}
	var o, e bytes.Buffer
	c := procgroup.Command{
		Class:  procgroup.ToolStep,
		Argv:   []string{sh, hook, "origin", "https://example.test/a b.git"},
		Dir:    dir,
		Env:    append(os.Environ(), "PATH="+path),
		Stdin:  strings.NewReader(stdin),
		Stdout: &o,
		Stderr: &e,
	}
	if link != "" {
		c.Dir = link
		c.Env = append(c.Env, "PWD="+link)
	}
	err = procgroup.Run(t.Context(), c)
	if exit, ok := errors.AsType[*procgroup.ExitError](err); ok {
		code = exit.Code
	} else {
		require.NoError(t, err)
	}
	return code, o.String(), e.String(), dir
}

func stubDir(t *testing.T, marks ...string) string {
	t.Helper()
	return stubDirIn(t, t.TempDir(), marks...)
}

// stubDirIn is stubDir in the directory bin, which it creates.
func stubDirIn(t *testing.T, bin string, marks ...string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(bin, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "go"), []byte(hookStub), 0o700))
	for _, m := range marks {
		require.NoError(t, os.WriteFile(filepath.Join(bin, m), nil, 0o600))
	}
	return bin
}

// TestHookExplainsAToolchainRefusal runs the tracked hook with a go
// stub that refuses the probe the way a go older than go.mod does: the
// hook lets the refusal through, adds what to do on the standard error
// (and prints nothing on the standard output), exits non-zero and never
// starts the checks. The hint depends on the go it finds: none, a mise
// shim, or another go. With a go that accepts the probe the hook starts
// the checks with the arguments of git and the refs git sends on the
// standard input, which the probe does not read; its exit status is the
// checks'. The stub is a shell script, which the four runners of CI run.
func TestHookExplainsAToolchainRefusal(t *testing.T) {
	const refusal = "go: go.mod requires go >= 9.9.9 (running go 1.0.0; GOTOOLCHAIN=local)"
	const refs = "refs/heads/main 1111111111111111111111111111111111111111 refs/heads/main 0000000000000000000000000000000000000000\n"
	t.Run("a go that is not a shim", func(t *testing.T) {
		bin := stubDir(t, "refuse")
		code, out, errOut, _ := runHook(t, bin, false, refs)
		assert.Equal(t, 1, code, "exit status\n%s", errOut)
		assert.Empty(t, out, "the standard output")
		assert.Contains(t, errOut, refusal)
		assert.Contains(t, errOut, "the go on PATH ("+filepath.Join(bin, "go")+") cannot run this module")
		assert.Contains(t, errOut, "put the mise shims first on PATH")
		assert.Contains(t, errOut, `eval "$(mise activate <shell> --shims)"`)
		assert.NotContains(t, errOut, "mise trust")
		assert.NotContains(t, errOut, "checks")
	})
	// A directory name that echo would expand: \c ends the output and
	// \033 is ESC.
	const hostile = `we\ird\c\033[2Jx`
	t.Run("a go that is not a shim, in a directory with escapes in its name", func(t *testing.T) {
		bin := stubDirIn(t, filepath.Join(t.TempDir(), hostile), "refuse")
		code, out, errOut, _ := runHook(t, bin, false, refs)
		assert.Equal(t, 1, code, "exit status\n%s", errOut)
		assert.Empty(t, out, "the standard output")
		assert.Contains(t, errOut, "the go on PATH ("+filepath.Join(bin, "go")+") cannot run this module",
			"the name is printed as it is")
		assert.NotContains(t, errOut, "\x1b")
		assert.Contains(t, errOut, "put the mise shims first on PATH", "the line after the name is still printed")
	})
	t.Run("a mise shim", func(t *testing.T) {
		shims := stubDirIn(t, filepath.Join(t.TempDir(), "mise", "shims"), "refuse")
		code, out, errOut, dir := runHook(t, shims, false, refs)
		assert.Equal(t, 1, code, "exit status\n%s", errOut)
		assert.Empty(t, out, "the standard output")
		assert.Contains(t, errOut, refusal)
		assert.Contains(t, errOut, `run "mise trust" and "mise install" in `+dir)
		assert.NotContains(t, errOut, "put the mise shims first")
	})
	t.Run("a mise shim, run in a symbolic link to a tree with escapes in its name", func(t *testing.T) {
		shims := stubDirIn(t, filepath.Join(t.TempDir(), "mise", "shims"), "refuse")
		real := filepath.Join(citest.MustEval(t, t.TempDir()), hostile)
		require.NoError(t, os.Mkdir(real, 0o700))
		link := filepath.Join(citest.MustEval(t, t.TempDir()), "tree")
		require.NoError(t, os.Symlink(real, link))
		code, _, errOut, _ := runHookIn(t, real, link, shims, false, refs)
		assert.Equal(t, 1, code, "exit status\n%s", errOut)
		assert.Contains(t, errOut, `run "mise trust" and "mise install" in `+real+",", "the resolved path, as it is")
		assert.NotContains(t, errOut, link)
		assert.NotContains(t, errOut, "\x1b")
	})
	t.Run("a go under shims that is not mise's", func(t *testing.T) {
		bin := stubDirIn(t, filepath.Join(t.TempDir(), "other", "shims"), "refuse")
		code, _, errOut, _ := runHook(t, bin, false, refs)
		assert.Equal(t, 1, code, "exit status\n%s", errOut)
		assert.Contains(t, errOut, "put the mise shims first on PATH")
		assert.NotContains(t, errOut, "mise trust")
	})
	t.Run("no go", func(t *testing.T) {
		bin := t.TempDir()
		for _, tool := range []string{"env", "dirname"} {
			p, err := procgroup.LookPath(tool)
			require.NoError(t, err)
			require.NoError(t, os.Symlink(p, filepath.Join(bin, tool)))
		}
		code, out, errOut, _ := runHook(t, bin, true, refs)
		assert.Equal(t, 1, code, "exit status\n%s", errOut)
		assert.Empty(t, out, "the standard output")
		assert.NotContains(t, errOut, "env:", "env is not started")
		assert.Contains(t, errOut, "pre-push: no go is on PATH.")
		assert.Contains(t, errOut, `eval "$(mise activate <shell> --shims)"`)
		assert.Contains(t, errOut, "see mise activate --help")
	})
	t.Run("a go that accepts the probe", func(t *testing.T) {
		bin := stubDir(t)
		code, out, errOut, _ := runHook(t, bin, false, refs)
		assert.Equal(t, 7, code, "the checks' exit status\n%s", errOut)
		assert.Equal(t, "checks run ./tools/ci fast origin https://example.test/a b.git\n"+refs, out,
			"the checks get the arguments and the refs of git, and the probe's output is dropped")
		assert.Empty(t, errOut)
		argv, err := os.ReadFile(filepath.Join(bin, "probe-argv"))
		require.NoError(t, err)
		assert.Equal(t, "list -m\n", string(argv), "the probe")
	})
}

// TestHookPassesSIGTERMToGo checks that a SIGTERM sent to the hook
// reaches the go that runs the checks, because the hook execs it, and that
// the hook's exit status is that go's. A shell writes its pid and execs
// the hook, so the pid the test signals is the hook's, and then the go
// stub's.
func TestHookPassesSIGTERMToGo(t *testing.T) {
	bin := stubDir(t, "hang")
	sh, err := procgroup.LookPath("sh")
	require.NoError(t, err)
	pidfile := filepath.Join(t.TempDir(), "pid")
	// The group is killed when the run ends, and a stub that never sees
	// SIGTERM is killed at this limit, so it does not outlive the test.
	ctx := procgroup.WithTestTimeout(t.Context(), procgroup.ToolStep, 30*time.Second)
	done := runAsync(ctx, procgroup.Command{
		Class: procgroup.ToolStep,
		Argv:  []string{sh, "-c", `echo $$ > "$0.tmp" && mv "$0.tmp" "$0" && exec "$1" "$2"`, pidfile, sh, citest.MustAbs(t, filepath.Join("..", "..", ".githooks", "pre-push"))},
		Dir:   citest.MustEval(t, t.TempDir()),
		Env:   append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH")),
	})
	pid := waitForPid(t, pidfile)
	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(bin, "ready"))
		return err == nil
	}, 10*time.Second, 20*time.Millisecond, "the checks started")
	require.NoError(t, syscall.Kill(pid, syscall.SIGTERM))
	select {
	case err := <-done:
		exit, ok := errors.AsType[*procgroup.ExitError](err)
		require.True(t, ok, "the hook ends with a status: %v", err)
		assert.Equal(t, 143, exit.Code, "the hook's status is the go stub's")
	case <-time.After(10 * time.Second):
		t.Fatal("the hook did not end after SIGTERM")
	}
	_, err = os.Stat(filepath.Join(bin, "term"))
	assert.NoError(t, err, "the go stub saw SIGTERM")
}
