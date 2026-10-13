// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errWriter is the error of failingWriter.
var errWriter = errors.New("the writer failed")

// failingWriter is a writer that fails every write.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWriter }

// TestWaitDelay pins the bound of the drain to the 2 s of 04 4.1,
// Subprocesses, item 4.
func TestWaitDelay(t *testing.T) {
	assert.Equal(t, 2*time.Second, WaitDelay)
}

// TestRun covers what a command gets and what Run returns: the
// environment is exactly Env, an empty one when Env is nil, standard
// input is Stdin or empty, each writer gets its own stream, the status
// of a command that failed is an *ExitError, and a program that is not
// there is ErrNotFound.
func TestRun(t *testing.T) {
	t.Run("the environment is exactly Env", func(t *testing.T) {
		t.Setenv("PROCGROUP_TEST_LEAK", "leaked")
		var out bytes.Buffer
		err := Run(t.Context(), Command{Class: JulietaGit, Argv: []string{"/usr/bin/env"}, Env: []string{"ONLY=this"}, Stdout: &out})
		require.NoError(t, err)
		assert.Equal(t, "ONLY=this\n", out.String())
	})
	t.Run("a nil Env is an empty environment", func(t *testing.T) {
		t.Setenv("PROCGROUP_TEST_LEAK", "leaked")
		var out bytes.Buffer
		err := Run(t.Context(), Command{Class: JulietaGit, Argv: []string{"/usr/bin/env"}, Stdout: &out})
		require.NoError(t, err)
		assert.Empty(t, out.String())
	})
	t.Run("standard input is Stdin", func(t *testing.T) {
		var out bytes.Buffer
		err := Run(t.Context(), Command{Class: JulietaGit, Argv: []string{"/bin/cat"}, Stdin: strings.NewReader("in\n"), Stdout: &out})
		require.NoError(t, err)
		assert.Equal(t, "in\n", out.String())
	})
	t.Run("a nil Stdin is empty and Dir is the working directory", func(t *testing.T) {
		dir := t.TempDir()
		var out bytes.Buffer
		err := Run(t.Context(), Command{Class: JulietaGit, Argv: []string{"/bin/sh", "-c", "cat; pwd -P"}, Env: shellEnv, Dir: dir, Stdout: &out})
		require.NoError(t, err)
		want, err := filepath.EvalSymlinks(dir)
		require.NoError(t, err)
		assert.Equal(t, want+"\n", out.String())
	})
	t.Run("Stdout and Stderr each get their own stream", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := Run(t.Context(), Command{Class: JulietaGit, Argv: []string{"/bin/sh", "-c", "echo out; echo err >&2"}, Env: shellEnv, Stdout: &stdout, Stderr: &stderr})
		require.NoError(t, err)
		assert.Equal(t, "out\n", stdout.String())
		assert.Equal(t, "err\n", stderr.String())
	})
	t.Run("one writer for both keeps the order", func(t *testing.T) {
		out, err := CombinedOutput(t.Context(), Command{Class: JulietaGit, Argv: []string{"/bin/sh", "-c", "echo a; echo b >&2; echo c"}, Env: shellEnv})
		require.NoError(t, err)
		assert.Equal(t, "a\nb\nc\n", string(out))
	})
	t.Run("a file writer is written directly", func(t *testing.T) {
		f, err := os.Create(filepath.Join(t.TempDir(), "out"))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, f.Close()) })
		err = Run(t.Context(), Command{Class: JulietaGit, Argv: []string{"/bin/sh", "-c", "echo file"}, Env: shellEnv, Stdout: f})
		require.NoError(t, err)
		b, err := os.ReadFile(f.Name())
		require.NoError(t, err)
		assert.Equal(t, "file\n", string(b))
	})
	t.Run("a writer that fails is the error of a command that succeeded", func(t *testing.T) {
		err := Run(t.Context(), Command{Class: JulietaGit, Argv: []string{"/bin/sh", "-c", "echo out"}, Env: shellEnv, Stdout: failingWriter{}})
		require.ErrorIs(t, err, errWriter)
	})
	t.Run("a failed command is an ExitError with its status", func(t *testing.T) {
		err := Run(t.Context(), Command{Class: JulietaGit, Argv: []string{"/bin/sh", "-c", "exit 3"}, Env: shellEnv})
		var exit *ExitError
		require.ErrorAs(t, err, &exit)
		assert.Equal(t, 3, exit.Code)
		assert.Zero(t, exit.Signal)
		assert.Equal(t, "/bin/sh exited 3", exit.Error())
		var execExit *exec.ExitError
		require.ErrorAs(t, err, &execExit, "Unwrap gives os/exec's error")
		assert.Equal(t, 3, execExit.ExitCode())
	})
	t.Run("a command a signal ended is an ExitError with the signal", func(t *testing.T) {
		err := Run(t.Context(), Command{Class: JulietaGit, Argv: []string{"/bin/sh", "-c", "kill -KILL $$"}, Env: shellEnv})
		var exit *ExitError
		require.ErrorAs(t, err, &exit)
		assert.Equal(t, -1, exit.Code)
		assert.Equal(t, syscall.SIGKILL, exit.Signal)
		assert.Equal(t, "/bin/sh was ended by signal 9 (killed)", exit.Error())
	})
	t.Run("a name that is not on PATH is ErrNotFound", func(t *testing.T) {
		err := Run(t.Context(), Command{Class: JulietaGit, Argv: []string{"procgroup-test-no-such-command"}})
		require.ErrorIs(t, err, ErrNotFound)
	})
	t.Run("an absolute path that is not there is ErrNotFound", func(t *testing.T) {
		err := Run(t.Context(), Command{Class: JulietaGit, Argv: []string{filepath.Join(t.TempDir(), "missing")}})
		require.ErrorIs(t, err, ErrNotFound)
	})
	t.Run("a relative path that is not there in Dir is ErrNotFound", func(t *testing.T) {
		err := Run(t.Context(), Command{Class: JulietaGit, Argv: []string{"./missing"}, Dir: t.TempDir()})
		require.ErrorIs(t, err, ErrNotFound)
	})
	t.Run("a working directory that is not there is not ErrNotFound", func(t *testing.T) {
		err := Run(t.Context(), Command{Class: JulietaGit, Argv: []string{"/bin/sh", "-c", "true"}, Dir: filepath.Join(t.TempDir(), "missing")})
		require.ErrorIs(t, err, fs.ErrNotExist)
		assert.NotErrorIs(t, err, ErrNotFound)
	})
	t.Run("an empty argv starts nothing", func(t *testing.T) {
		err := Run(t.Context(), Command{Class: JulietaGit})
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
// grandchild cannot ignore, the group is killed as soon as the leader
// ends, and the drain after the kill is bounded by WaitDelay.
func TestRunKillsTheGroup(t *testing.T) {
	t.Run("a cancellation kills a grandchild that ignores SIGTERM", func(t *testing.T) {
		pidfile := filepath.Join(t.TempDir(), "pid")
		killAtEnd(t, pidfile)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		errc := make(chan error, 1)
		go func() {
			_, err := CombinedOutput(ctx, Command{
				Class: Unbounded,
				Argv:  []string{"/bin/sh", "-c", `trap "" TERM; sleep 60 & echo $! > "$0"; wait`, pidfile},
				Env:   shellEnv,
			})
			errc <- err
		}()
		pid := waitForPid(t, pidfile)
		start := time.Now()
		cancel()
		err := <-errc
		require.ErrorIs(t, err, context.Canceled)
		assert.Less(t, time.Since(start), WaitDelay, "the group died with the pipe, so no drain was waited out")
		assertGone(t, pid)
	})
	t.Run("a grandchild holding the pipe is dead after the step returns", func(t *testing.T) {
		// os/exec kills the group only while the leader runs, so a leader
		// that exits at once leaves the grandchild, which holds the
		// output pipe, to Run.
		pidfile := filepath.Join(t.TempDir(), "pid")
		killAtEnd(t, pidfile)
		start := time.Now()
		out, err := CombinedOutput(t.Context(), Command{Class: JulietaGit, Argv: []string{"/bin/sh", "-c", `sleep 60 & echo $! > "$0"; echo out; exit 3`, pidfile}, Env: shellEnv})
		assert.Less(t, time.Since(start), WaitDelay, "the group kill closed the pipe; the drain was not waited out")
		var exit *ExitError
		require.ErrorAs(t, err, &exit)
		assert.Equal(t, 3, exit.Code)
		assert.Equal(t, "out\n", string(out))
		assertGone(t, waitForPid(t, pidfile))
	})
	t.Run("a command that exits 0 while a grandchild holds the pipe succeeds", func(t *testing.T) {
		pidfile := filepath.Join(t.TempDir(), "pid")
		killAtEnd(t, pidfile)
		start := time.Now()
		out, err := CombinedOutput(t.Context(), Command{Class: JulietaGit, Argv: []string{"/bin/sh", "-c", `sleep 60 & echo $! > "$0"; echo out`, pidfile}, Env: shellEnv})
		require.NoError(t, err)
		assert.Less(t, time.Since(start), WaitDelay)
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
		require.ErrorIs(t, <-errc, context.Canceled)
		assert.Less(t, time.Since(start), bound, "the command returns after WaitDelay, not when the holder ends")
		assert.NoError(t, syscall.Kill(holder, 0), "the holder is still alive, so only WaitDelay ended the wait")
	})
	t.Run("a leader that exits 0 with a holder outside the group succeeds after WaitDelay", func(t *testing.T) {
		cmd, pidfile, _ := helper(t, "escape-exit")
		start := time.Now()
		_, err := CombinedOutput(t.Context(), Command{Class: Unbounded, Argv: cmd.Args, Env: cmd.Env})
		require.NoError(t, err)
		took := time.Since(start)
		assert.GreaterOrEqual(t, took, WaitDelay, "the drain waited for the holder")
		assert.Less(t, took, WaitDelay+3*time.Second, "and no longer than its bound")
		assert.NoError(t, syscall.Kill(waitForPid(t, pidfile), 0), "the holder is still alive")
	})
}

// recordKills replaces kill with a recorder that returns result, and
// returns the pids it was called with. No signal is sent.
func recordKills(t *testing.T, result error) *[]int {
	t.Helper()
	var calls []int
	real := kill
	kill = func(pid int, sig syscall.Signal) error {
		assert.Equal(t, syscall.SIGKILL, sig)
		calls = append(calls, pid)
		return result
	}
	t.Cleanup(func() { kill = real })
	return &calls
}

// TestKillGroup checks the target and the result of the kill: the
// group, by its negative pid; ESRCH, a group already gone, is
// os.ErrProcessDone and other errors stay; and a pid at or below 1, or
// the caller's own group, is refused without a signal. The refusals run
// against a recorder, so a broken guard sends nothing.
func TestKillGroup(t *testing.T) {
	t.Run("the group is killed by its negative pid", func(t *testing.T) {
		calls := recordKills(t, nil)
		require.NoError(t, KillGroup(4242))
		assert.Equal(t, []int{-4242}, *calls)
	})
	t.Run("ESRCH is os.ErrProcessDone", func(t *testing.T) {
		recordKills(t, syscall.ESRCH)
		require.ErrorIs(t, KillGroup(4242), os.ErrProcessDone)
	})
	t.Run("EPERM stays an error", func(t *testing.T) {
		recordKills(t, syscall.EPERM)
		err := KillGroup(4242)
		require.ErrorIs(t, err, syscall.EPERM)
		assert.NotErrorIs(t, err, os.ErrProcessDone)
	})
	refused := map[string]int{"pid 1": 1, "pid 0": 0, "pid -1": -1, "pid -2": -2, "the caller's own group": syscall.Getpgrp()}
	for name, pid := range refused {
		t.Run(name+" is refused", func(t *testing.T) {
			calls := recordKills(t, nil)
			require.ErrorIs(t, KillGroup(pid), errNotAGroup)
			assert.Empty(t, *calls)
		})
	}
	t.Run("a group that is gone is os.ErrProcessDone, at each kill", func(t *testing.T) {
		var out bytes.Buffer
		require.NoError(t, Run(t.Context(), Command{Class: JulietaGit, Argv: []string{"/bin/sh", "-c", "echo $$"}, Env: shellEnv, Stdout: &out}))
		pid, err := strconv.Atoi(strings.TrimSpace(out.String()))
		require.NoError(t, err)
		require.Greater(t, pid, 1, "a child's pid")
		require.ErrorIs(t, KillGroup(pid), os.ErrProcessDone)
		require.ErrorIs(t, KillGroup(pid), os.ErrProcessDone)
	})
}

// TestGroups covers the record of the running groups that Main's
// handler kills: a start that fails records nothing and returns its
// error, and starts, ends and the handler's kill run at once without a
// race (go test -race).
func TestGroups(t *testing.T) {
	t.Run("a start that fails is the error and records nothing", func(t *testing.T) {
		g := &groups{}
		ctx := context.WithValue(t.Context(), groupsKey{}, g)
		err := Run(ctx, Command{Class: JulietaGit, Argv: []string{"procgroup-test-no-such-command"}})
		require.ErrorIs(t, err, ErrNotFound)
		assert.Empty(t, g.leaders)
	})
	t.Run("starts, ends and the kill run at once", func(t *testing.T) {
		g := &groups{}
		ctx := context.WithValue(t.Context(), groupsKey{}, g)
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				// Either the command ran, or the kill came first: both are
				// fine, the race detector judges the record.
				_ = Run(ctx, Command{Class: JulietaGit, Argv: []string{"/bin/sh", "-c", "sleep 0.05"}, Env: shellEnv})
			})
		}
		time.Sleep(20 * time.Millisecond)
		g.killAll()
		wg.Wait()
		g.mu.Lock()
		defer g.mu.Unlock()
		assert.Empty(t, g.leaders)
		assert.True(t, g.isClosed)
	})
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
		err := Run(ctx, Command{Class: JulietaGit, Argv: []string{"/bin/sh", "-c", `echo > "$0"`, marker}, Env: shellEnv})
		require.ErrorIs(t, err, context.Canceled)
		assert.NoFileExists(t, marker)
	})
}
