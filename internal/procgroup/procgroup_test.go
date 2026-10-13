// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRun covers what a command gets and what Run returns: the
// environment is exactly Env, an empty one when Env is nil, standard
// input is empty, the status of a command that failed is an
// *ExitError, and a command that is not there is ErrNotFound.
func TestRun(t *testing.T) {
	t.Run("the environment is exactly Env", func(t *testing.T) {
		t.Setenv("PROCGROUP_TEST_LEAK", "leaked")
		var out bytes.Buffer
		err := Run(t.Context(), Command{Class: ToolQuery, Argv: []string{"/usr/bin/env"}, Env: []string{"ONLY=this"}, Stdout: &out})
		require.NoError(t, err)
		assert.Equal(t, "ONLY=this\n", out.String())
	})
	t.Run("a nil Env is an empty environment", func(t *testing.T) {
		t.Setenv("PROCGROUP_TEST_LEAK", "leaked")
		var out bytes.Buffer
		err := Run(t.Context(), Command{Class: ToolQuery, Argv: []string{"/usr/bin/env"}, Stdout: &out})
		require.NoError(t, err)
		assert.Empty(t, out.String())
	})
	t.Run("standard input is empty and Dir is the working directory", func(t *testing.T) {
		dir := t.TempDir()
		var out bytes.Buffer
		err := Run(t.Context(), Command{Class: ToolQuery, Argv: []string{"/bin/sh", "-c", "cat; pwd -P"}, Env: shellEnv, Dir: dir, Stdout: &out})
		require.NoError(t, err)
		want, err := filepath.EvalSymlinks(dir)
		require.NoError(t, err)
		assert.Equal(t, want+"\n", out.String())
	})
	t.Run("a failed command is an ExitError with its status", func(t *testing.T) {
		err := Run(t.Context(), Command{Class: ToolQuery, Argv: []string{"/bin/sh", "-c", "exit 3"}, Env: shellEnv})
		var exit *ExitError
		require.ErrorAs(t, err, &exit)
		assert.Equal(t, 3, exit.Code)
		assert.Zero(t, exit.Signal)
		assert.Equal(t, "/bin/sh exited 3", exit.Error())
	})
	t.Run("a command a signal ended is an ExitError with the signal", func(t *testing.T) {
		err := Run(t.Context(), Command{Class: ToolQuery, Argv: []string{"/bin/sh", "-c", "kill -KILL $$"}, Env: shellEnv})
		var exit *ExitError
		require.ErrorAs(t, err, &exit)
		assert.Equal(t, -1, exit.Code)
		assert.Equal(t, syscall.SIGKILL, exit.Signal)
		assert.Equal(t, "/bin/sh was ended by signal 9 (killed)", exit.Error())
	})
	t.Run("a command that is not there is ErrNotFound", func(t *testing.T) {
		err := Run(t.Context(), Command{Class: ToolQuery, Argv: []string{"procgroup-test-no-such-command"}})
		require.ErrorIs(t, err, ErrNotFound)
	})
	t.Run("an empty argv starts nothing", func(t *testing.T) {
		err := Run(t.Context(), Command{Class: ToolQuery})
		require.ErrorIs(t, err, errNoArgv)
	})
	t.Run("a command with no class starts nothing", func(t *testing.T) {
		pidfile := filepath.Join(t.TempDir(), "pid")
		err := Run(t.Context(), Command{Argv: []string{"/bin/sh", "-c", `echo $$ > "$0"`, pidfile}, Env: shellEnv})
		require.ErrorIs(t, err, errNoClass)
		assert.NoFileExists(t, pidfile)
	})
}

// TestRunKillsTheGroup covers the group rule of 04 4.1, Subprocesses,
// item 4: a cancellation kills the whole group with the SIGKILL a
// grandchild cannot ignore, the group is killed when the leader ends,
// and the drain after the kill is bounded by WaitDelay.
func TestRunKillsTheGroup(t *testing.T) {
	t.Run("a cancellation kills a grandchild that ignores SIGTERM", func(t *testing.T) {
		pidfile := filepath.Join(t.TempDir(), "pid")
		killAtEnd(t, pidfile)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		errc := make(chan error, 1)
		go func() {
			errc <- Run(ctx, Command{
				Class: Unbounded,
				Argv:  []string{"/bin/sh", "-c", `trap "" TERM; sleep 60 & echo $! > "$0"; wait`, pidfile},
				Env:   shellEnv,
			})
		}()
		pid := waitForPid(t, pidfile)
		start := time.Now()
		cancel()
		err := <-errc
		require.ErrorIs(t, err, context.Canceled)
		assert.Less(t, time.Since(start), 5*time.Second)
		assertGone(t, pid)
	})
	t.Run("a grandchild holding the pipe is dead after the step returns", func(t *testing.T) {
		// os/exec kills the group only while the leader runs, so a leader
		// that exits at once leaves the grandchild to Run.
		pidfile := filepath.Join(t.TempDir(), "pid")
		killAtEnd(t, pidfile)
		script := `sleep 60 >/dev/null 2>&1 & echo $! > "$0"; echo out; exit 3`
		out, err := CombinedOutput(t.Context(), Command{Class: ToolStep, Argv: []string{"/bin/sh", "-c", script, pidfile}, Env: shellEnv})
		var exit *ExitError
		require.ErrorAs(t, err, &exit)
		assert.Equal(t, 3, exit.Code)
		assert.Equal(t, "out\n", string(out))
		assertGone(t, waitForPid(t, pidfile))
	})
	t.Run("a holder of the pipe outside the group is waited for no longer than WaitDelay", func(t *testing.T) {
		cmd, pidfile, _ := helper(t, "escape")
		const bound = 6 * time.Second // the holder sleeps for 15s
		require.Less(t, WaitDelay, bound-time.Second, "the bound of the wait is under the bound of the test")
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		errc := make(chan error, 1)
		go func() {
			_, err := CombinedOutput(ctx, Command{Class: Unbounded, Argv: cmd.Args, Env: cmd.Env})
			errc <- err
		}()
		// The holder exists before the command is cancelled, so the wait
		// that follows is for a live holder and not for nothing.
		holder := waitForPid(t, pidfile)
		start := time.Now()
		cancel()
		require.Error(t, <-errc)
		assert.Less(t, time.Since(start), bound, "the command returns after WaitDelay, not when the holder ends")
		assert.NoError(t, syscall.Kill(holder, 0), "the holder is still alive, so only WaitDelay ended the wait")
	})
}

// TestKillGroup checks that a group that is already gone is not an
// error, and that a pid at or below 1 is refused:
// -1 would signal every process this user may signal, and -0 the
// caller's own group.
func TestKillGroup(t *testing.T) {
	t.Run("a group that is gone is os.ErrProcessDone", func(t *testing.T) {
		var out bytes.Buffer
		require.NoError(t, Run(t.Context(), Command{Class: ToolQuery, Argv: []string{"/bin/sh", "-c", "echo $$"}, Env: shellEnv, Stdout: &out}))
		pid, err := strconv.Atoi(strings.TrimSpace(out.String()))
		require.NoError(t, err)
		require.ErrorIs(t, KillGroup(pid), os.ErrProcessDone)
	})
	for _, pid := range []int{1, 0, -1, -2} {
		t.Run("pid "+strconv.Itoa(pid)+" is refused", func(t *testing.T) {
			require.ErrorIs(t, KillGroup(pid), errNotAGroup)
		})
	}
}

// TestInterrupt covers the order of the handler's first signal: cancel,
// then the synchronous kill of every running group, then Stop, so a
// second signal that ends the process by its default action finds no
// group left to orphan. The group here runs under a context the cancel
// does not reach, so only the handler's own kill can end it; stop
// checks that it is dead before stop runs.
func TestInterrupt(t *testing.T) {
	groups := &groups{}
	pidfile := filepath.Join(t.TempDir(), "pid")
	killAtEnd(t, pidfile)
	ctx := context.WithValue(t.Context(), groupsKey{}, groups)
	errc := make(chan error, 1)
	go func() {
		errc <- Run(ctx, Command{Class: Unbounded, Argv: []string{"/bin/sh", "-c", grandchildScript, pidfile}, Env: shellEnv})
	}()
	pid := waitForPid(t, pidfile)
	var cancelled, stopped bool
	interrupt(func() { cancelled = true }, groups, func() {
		assert.True(t, cancelled, "cancel runs before stop")
		assertGone(t, pid)
		stopped = true
	})
	assert.True(t, stopped)
	require.Error(t, <-errc)

	t.Run("a start after the interrupt starts nothing", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "started")
		err := Run(ctx, Command{Class: ToolQuery, Argv: []string{"/bin/sh", "-c", `echo > "$0"`, marker}, Env: shellEnv})
		require.ErrorIs(t, err, context.Canceled)
		assert.NoFileExists(t, marker)
	})
}
