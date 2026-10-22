// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// script writes an executable sh script as the fake sbx of a test and
// returns its path.
func script(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sbx")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755))
	return path
}

// runner returns a Runner for the script.
func runner(sbx string) *Runner {
	return &Runner{Sbx: sbx, Home: "/home/test", Term: "xterm-test"}
}

// TestRunPassesTheArgvAndTheScrubbedEnvironment holds 04 4.1: sbx runs
// by the absolute path with the argv of the call, and with nothing of
// this process's environment.
func TestRunPassesTheArgvAndTheScrubbedEnvironment(t *testing.T) {
	t.Setenv("FOO", "leaks")
	t.Setenv("SSH_AUTH_SOCK", "/leaks")
	sbx := script(t, `for a in "$@"; do echo "arg:$a"; done
echo "PATH=$PATH HOME=$HOME LANG=$LANG FOO=${FOO-unset} SOCK=${SSH_AUTH_SOCK-unset} TERM=${TERM-unset}"`)
	res, err := runner(sbx).Run(context.Background(), Version(), Opts{})
	require.NoError(t, err)
	assert.Equal(t, "arg:version\nPATH=/usr/bin:/bin HOME=/home/test LANG=C FOO=unset SOCK=unset TERM=unset\n", string(res.Stdout))
	assert.False(t, res.Truncated)
}

// TestRunInteractive gives an interactive call the terminal's streams
// and TERM.
func TestRunInteractive(t *testing.T) {
	sbx := script(t, `read line; echo "got:$line TERM=$TERM"; echo oops >&2`)
	var out, errOut bytes.Buffer
	r := runner(sbx)
	r.Terminal = Terminal{In: strings.NewReader("hello\n"), Out: &out, Err: &errOut}
	c := Call{Name: "test", Argv: []string{"x"}, Class: procgroup.SbxStart, Mode: Interactive}
	res, err := r.Run(context.Background(), c, Opts{})
	require.NoError(t, err)
	assert.Equal(t, "got:hello TERM=xterm-test\n", out.String())
	assert.Equal(t, "oops\n", errOut.String())
	assert.Empty(t, res.Stdout, "an interactive call captures nothing")
}

// TestRunRefusesWhatItCannotRun starts nothing for a relative path, a
// call without a mode and a call that breaks a guard.
func TestRunRefusesWhatItCannotRun(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	sbx := script(t, `touch `+marker)
	tests := []struct {
		name string
		r    *Runner
		c    Call
	}{
		{"a relative path", runner("sbx"), Version()},
		{"an unclean path", runner(sbx + "/../sbx"), Version()},
		{"no mode", runner(sbx), Call{Name: "x", Argv: []string{"x"}, Class: procgroup.SbxRead}},
		{"auto-approve", runner(sbx), Call{Name: "x", Argv: []string{"env", "run", "--auto-approve"}, Class: procgroup.SbxStart, Mode: Interactive}},
		{"a secret by value", runner(sbx), Call{Name: "x", Argv: []string{"secret", "set", "github", "-t", "x"}, Class: procgroup.SbxChange, Mode: Captured}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.r.Run(context.Background(), tt.c, Opts{})
			testsupport.RequireID(t, "RJ-348", err)
			assert.NoFileExists(t, marker)
		})
	}
}

// TestRunFailureDetails is the details table of T029: a failed call
// returns the escaped argv, the exit status, the head of stderr with the
// host's names replaced, the step and the three versions.
func TestRunFailureDetails(t *testing.T) {
	sbx := script(t, `echo "open /home/agent/dev/shop-env: permission denied for agent on agent-mbp" >&2; echo "\033[31mred" >&2; exit 3`)
	r := runner(sbx)
	r.Scrub = Scrubber{Home: "/home/agent", User: "agent", Host: "agent-mbp", Root: "/home/agent/dev"}
	r.Versions = Versions{Romeu: "v1.2.3", Julieta: "v1.2.4", Sbx: "0.48.0"}
	c := Call{Name: "test", Argv: []string{"env", "rm", "/home/agent/dev/shop env"}, Class: procgroup.SbxChange, Mode: Captured}
	_, err := r.Run(context.Background(), c, Opts{Step: "run step 3"})
	ce, ok := errors.AsType[*CallError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, []cli.Detail{
		{Key: "argv", Value: `sbx env rm "/home/agent/dev/shop env"`},
		{Key: "status", Value: "exit 3"},
		{Key: "stderr", Value: `open <root>/shop-env: permission denied for <user> on <host>\n\u001b[31mred\n`},
		{Key: "step", Value: "run step 3"},
		{Key: "romeu version", Value: "v1.2.3"},
		{Key: "julieta version", Value: "v1.2.4"},
		{Key: "sbx version", Value: "0.48.0"},
	}, ce.Details())
	assert.Equal(t, "sbx test failed: exit 3", ce.Error())
	_, isExit := errors.AsType[*procgroup.ExitError](err)
	assert.True(t, isExit, "the error of procgroup is in the chain")
}

// TestRunFailureDetailsUnknowns prints an unknown version as unknown and
// leaves out a step the caller did not give.
func TestRunFailureDetailsUnknowns(t *testing.T) {
	_, err := runner(script(t, `exit 1`)).Run(context.Background(), Version(), Opts{})
	ce, ok := errors.AsType[*CallError](err)
	require.True(t, ok)
	keys := []string{}
	for _, d := range ce.Details() {
		keys = append(keys, d.Key)
	}
	assert.Equal(t, []string{"argv", "status", "stderr", "romeu version", "julieta version", "sbx version"}, keys)
	assert.Contains(t, ce.Details(), cli.Detail{Key: "sbx version", Value: "unknown"})
}

// TestRunStderrHead cuts the stderr at 4 KiB, on the start of a
// character, after the host's names are replaced.
func TestRunStderrHead(t *testing.T) {
	sbx := script(t, `i=0; while [ $i -lt 300 ]; do printf 'agent /home/agent 0123456789012345678901234567890123456789\n' >&2; i=$((i+1)); done; exit 1`)
	r := runner(sbx)
	r.Scrub = Scrubber{Home: "/home/agent", User: "agent"}
	_, err := r.Run(context.Background(), Version(), Opts{})
	ce, _ := errors.AsType[*CallError](err)
	require.NotNil(t, ce)
	var stderr string
	for _, d := range ce.Details() {
		if d.Key == "stderr" {
			stderr = d.Value
		}
	}
	assert.LessOrEqual(t, len(stderr), StderrHead+len("\\n")*100, "the head is cut at 4 KiB before the line feeds are written as escapes")
	assert.NotContains(t, stderr, "agent /Users")
	assert.True(t, strings.HasPrefix(stderr, "<user> <home> 0123"))
}

// TestHeadCutsOnACharacter does not split a character at the cut.
func TestHeadCutsOnACharacter(t *testing.T) {
	text := strings.Repeat("a", StderrHead-1) + "é" + "tail"
	got := head([]byte(text), Scrubber{})
	assert.Equal(t, strings.Repeat("a", StderrHead-1), got)
	assert.Equal(t, strings.Repeat("a", StderrHead), head([]byte(strings.Repeat("a", StderrHead+10)), Scrubber{}))
}

// TestRunTimeout is the proof of T029 that subprocess-timeout (RJ-323)
// fires through the seam of the timeout table: the limit of the class is
// shortened for this context only, and the call that passes it fails
// with the timeout of procgroup in the chain.
func TestRunTimeout(t *testing.T) {
	sbx := script(t, `sleep 30`)
	ctx := procgroup.WithTestTimeout(context.Background(), procgroup.SbxRead, 200*time.Millisecond)
	start := time.Now()
	_, err := runner(sbx).Run(ctx, Version(), Opts{Step: "preflight step 1"})
	require.Error(t, err)
	assert.Less(t, time.Since(start), 10*time.Second)
	te, ok := errors.AsType[*procgroup.TimeoutError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, "RJ-323", te.ID())
	assert.Equal(t, "subprocess-timeout", te.Slug())
	assert.Equal(t, procgroup.SbxRead, te.Class)
	ce, ok := errors.AsType[*CallError](err)
	require.True(t, ok)
	assert.Contains(t, ce.Details(), cli.Detail{Key: "status", Value: "timed out: " + te.Details()})
	assert.Contains(t, ce.Details(), cli.Detail{Key: "step", Value: "preflight step 1"})
}

// TestRunTimeoutClasses holds that each builder runs under its own
// class: a read is cut at the limit of sbx-read and a create is not.
func TestRunTimeoutClasses(t *testing.T) {
	sbx := script(t, `sleep 1`)
	ctx := procgroup.WithTestTimeout(context.Background(), procgroup.SbxRead, 100*time.Millisecond)
	create := Call{Name: "create", Argv: []string{"x"}, Class: procgroup.SbxCreate, Mode: Captured}
	_, err := runner(sbx).Run(ctx, create, Opts{})
	assert.NoError(t, err, "the limit of sbx-read does not reach a call of sbx-create")
}

// TestRunNotFound is an sbx that is not there.
func TestRunNotFound(t *testing.T) {
	_, err := runner("/nonexistent/dir/sbx").Run(context.Background(), Version(), Opts{})
	ce, ok := errors.AsType[*CallError](err)
	require.True(t, ok, "%v", err)
	assert.Contains(t, ce.Details(), cli.Detail{Key: "status", Value: "sbx was not found"})
	assert.ErrorIs(t, err, procgroup.ErrNotFound)
}

// TestRunSignal reports a call a signal ended.
func TestRunSignal(t *testing.T) {
	_, err := runner(script(t, `kill -9 $$`)).Run(context.Background(), Version(), Opts{})
	ce, ok := errors.AsType[*CallError](err)
	require.True(t, ok)
	assert.Contains(t, ce.Details(), cli.Detail{Key: "status", Value: "ended by signal 9 (killed)"})
}

// TestRunCancelled returns the cancellation, not a CallError: the
// signal rule owns it.
func TestRunCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	_, err := runner(script(t, `sleep 30`)).Run(ctx, Version(), Opts{})
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
	_, isCall := errors.AsType[*CallError](err)
	assert.False(t, isCall)
}

// TestRunOutputCap keeps the first bytes of a runaway output and says so.
func TestRunOutputCap(t *testing.T) {
	var b capped
	b.max = 4
	n, err := b.Write([]byte("abcdef"))
	require.NoError(t, err)
	assert.Equal(t, 6, n, "the writer takes what the command wrote")
	assert.Equal(t, "abcd", b.buf.String())
	assert.True(t, b.full)
	n, err = b.Write([]byte("gh"))
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, "abcd", b.buf.String())
}

// TestRunContributionsReachTheCall passes a registered contribution to
// the call through the runner.
func TestRunContributionsReachTheCall(t *testing.T) {
	saved := slices.Clone(contributions)
	t.Cleanup(func() { contributions = saved })
	register("probe", func(in EnvInput) []string { return []string{"PROBE_VAR=" + in.Call.Name} })
	res, err := runner(script(t, `echo "$PROBE_VAR"`)).Run(context.Background(), Version(), Opts{})
	require.NoError(t, err)
	assert.Equal(t, "version\n", string(res.Stdout))
}

// TestRunnerVersion reads the version through the runner.
func TestRunnerVersion(t *testing.T) {
	v, err := runner(script(t, `echo v0.48.0`)).Version(context.Background(), Opts{})
	require.NoError(t, err)
	assert.Equal(t, "0.48.0", v.String())

	_, err = runner(script(t, `echo not a version`)).Version(context.Background(), Opts{})
	testsupport.RequireID(t, "RJ-310", err)

	_, err = runner(script(t, `exit 1`)).Version(context.Background(), Opts{})
	_, isCall := errors.AsType[*CallError](err)
	assert.True(t, isCall)
}

// TestRequireFloor is the sbx floor check before gate 1 (REQ-0441): a
// version below the floor is sbx-floor, and one at it passes.
func TestRequireFloor(t *testing.T) {
	v, err := runner(script(t, `echo v0.46.0`)).RequireFloor(context.Background(), Opts{})
	require.NoError(t, err)
	assert.Equal(t, "0.46.0", v.String())

	v, err = runner(script(t, `echo v0.45.3`)).RequireFloor(context.Background(), Opts{})
	testsupport.RequireID(t, "RJ-343", err)
	assert.Equal(t, "0.45.3", v.String(), "the version found is returned with the error")
	assert.Equal(t, 2, cli.ExitCode(err))

	_, err = runner(script(t, `exit 1`)).RequireFloor(context.Background(), Opts{})
	require.Error(t, err)
	assert.NotEqual(t, "RJ-343", cli.Of(err).ID)
}
