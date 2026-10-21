// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks_test

import (
	"bytes"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/hooks"
	ihooks "github.com/brunovenceslau/romeu-e-julieta/internal/hooks"
)

var clock = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

// sandbox is a HOME and a repo whose working directory the test is in.
type sandbox struct {
	home, repo string
	exits      []int
}

func newSandbox(t *testing.T, stdin string) *sandbox {
	t.Helper()
	s := &sandbox{home: t.TempDir(), repo: t.TempDir()}
	t.Setenv("HOME", s.home)
	require.NoError(t, os.MkdirAll(filepath.Join(s.repo, ".git", "hooks"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(s.repo, ".githooks"), 0o755))
	t.Chdir(s.repo)
	t.Cleanup(hooks.Process(stdin, []string{"PATH=/usr/bin:/bin", "MARK=seen"}, func(code int) { s.exits = append(s.exits, code) }))
	return s
}

func (s *sandbox) hook(t *testing.T, rel, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(s.repo, rel), []byte("#!/bin/sh\n"+body+"\n"), 0o755))
}

// julieta runs julieta with args and returns the exit status of
// cli.Main, stdout and stderr.
func julieta(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "julieta", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr), Now: func() time.Time { return clock }}
	code := cli.Main(t.Context(), args, env)
	return code, stdout.String(), stderr.String()
}

func TestRunPassesArgumentsStdinAndEnvironmentToTheHook(t *testing.T) {
	s := newSandbox(t, "stdin text")
	out := filepath.Join(s.repo, "out")
	s.hook(t, ".githooks/commit-msg", `echo "$MARK|$*|$(cat)" >`+out)
	code, stdout, stderr := julieta(t, "hooks", "run", "commit-msg", "-x", "--file=m", "--json", "-h")
	assert.Equal(t, 0, code, stderr)
	assert.Empty(t, stdout+stderr)
	assert.Empty(t, s.exits)
	b, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, "seen|-x --file=m --json -h|stdin text\n", string(b), "flags after the hook are its arguments")
}

// TestRunGivesEachRunItsStdin holds the stdin seam to the process it
// stands for: each run reads all of its stdin, as each process has its
// own. A reader shared by two runs leaves the second one empty, and its
// stdin copy, which procgroup.Run does not wait for, can still be
// reading it when the second run starts (a data race).
func TestRunGivesEachRunItsStdin(t *testing.T) {
	s := newSandbox(t, "stdin text")
	out := filepath.Join(s.repo, "out")
	s.hook(t, ".githooks/pre-push", `cat >>`+out+`; echo >>`+out)
	for range 2 {
		code, _, stderr := julieta(t, "hooks", "run", "pre-push")
		require.Equal(t, 0, code, stderr)
	}
	b, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, "stdin text\nstdin text\n", string(b))
}

func TestRunEndsWithTheStatusOfTheStageThatFailed(t *testing.T) {
	s := newSandbox(t, "")
	s.hook(t, ".githooks/pre-commit", `echo "from the hook"; echo "and its stderr" >&2; exit 5`)
	code, stdout, stderr := julieta(t, "hooks", "run", "pre-commit")
	assert.Equal(t, 0, code, "the status is the process's, through the exit seam")
	assert.Equal(t, []int{5}, s.exits)
	assert.Equal(t, "from the hook\n", stdout)
	assert.Contains(t, stderr, "and its stderr\n")
	assert.Contains(t, stderr, "julieta hooks: pre-commit stage tracked ("+s.repo+"/.githooks/pre-commit) exited 5\n")
	assert.NotContains(t, stderr, "error RJ-", "no second error block")

	// The failure is in julieta state, below HOME, until pre-commit passes.
	got, err := ihooks.Failures(filepath.Join(s.home, ".local/state/julieta/hook-failures"))
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 5, got[0].ExitCode)
	assert.Equal(t, clock, got[0].At)

	s.hook(t, ".githooks/pre-commit", "exit 0")
	code, _, _ = julieta(t, "hooks", "run", "pre-commit")
	assert.Equal(t, 0, code)
	got, err = ihooks.Failures(filepath.Join(s.home, ".local/state/julieta/hook-failures"))
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestRunRefusesAHookItDoesNotRun(t *testing.T) {
	newSandbox(t, "")
	code, _, stderr := julieta(t, "hooks", "run", "../pre-commit", "--json")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "RJ-349")
	code, _, _ = julieta(t, "hooks", "run")
	assert.Equal(t, 2, code, "no hook is a usage error")
}

func TestRunReadsNoManifest(t *testing.T) {
	// A commit must not depend on a manifest: the dispatcher needs only HOME.
	s := newSandbox(t, "")
	s.hook(t, ".githooks/post-commit", "exit 0")
	t.Setenv("JULIETA_MANIFEST", "")
	code, _, stderr := julieta(t, "hooks", "run", "post-commit")
	assert.Equal(t, 0, code, stderr)
	_, err := os.Stat(filepath.Join(s.home, ".local/state/julieta/manifest.json"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestDocumentOfFailureIsStrictJSON(t *testing.T) {
	s := newSandbox(t, "")
	s.hook(t, ".githooks/pre-push", "exit 2")
	julieta(t, "hooks", "run", "pre-push")
	files, err := filepath.Glob(filepath.Join(s.home, ".local/state/julieta/hook-failures/*.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	b, err := os.ReadFile(files[0])
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	assert.Len(t, m, 6)
}
