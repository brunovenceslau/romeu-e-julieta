// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// lateWriter is the reader of a loaded machine: its first write returns
// only after a pause longer than WaitDelay.
type lateWriter struct {
	buf   bytes.Buffer
	slept bool
}

func (w *lateWriter) Write(b []byte) (int, error) {
	if !w.slept {
		w.slept = true
		time.Sleep(WaitDelay + 500*time.Millisecond)
	}
	return w.buf.Write(b)
}

// TestWaitDelay pins the idle bound of the drain to the 2 s of 04 4.1,
// Subprocesses, item 4.
func TestWaitDelay(t *testing.T) {
	assert.Equal(t, 2*time.Second, WaitDelay)
	assert.Equal(t, 30*time.Second, drainCap, "the cap of the drain is the 30 s of the same item")
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
	t.Run("a late reader gets the whole output of a command that ended", func(t *testing.T) {
		// 64 KiB fits the pipe, so the command ends with most of it
		// still unread while the first write waits past WaitDelay.
		const size = 64 * 1024
		w := &lateWriter{}
		err := Run(t.Context(), Command{Class: Unbounded, Argv: []string{"/bin/sh", "-c", "head -c 65536 /dev/zero"}, Env: shellEnv, Stdout: w})
		require.NoError(t, err)
		assert.Equal(t, size, w.buf.Len(), "nothing the command wrote is lost")
	})
	t.Run("a writer that fails stops the command well inside its limit", func(t *testing.T) {
		// More than a pipe holds: with the copy stopped and its end open,
		// the command would block until its timeout.
		const limit = 10 * time.Second
		ctx := WithTestTimeout(t.Context(), SbxRead, limit)
		start := time.Now()
		err := Run(ctx, Command{Class: SbxRead, Argv: []string{"/bin/sh", "-c", "head -c 1000000 /dev/zero"}, Env: shellEnv, Stdout: failingWriter{}})
		require.Error(t, err)
		assert.Less(t, time.Since(start), limit/2)
		var timeout *TimeoutError
		assert.NotErrorAs(t, err, &timeout)
		var exit *ExitError
		isExit := errors.As(err, &exit)
		assert.True(t, errors.Is(err, errWriter) || isExit && (exit.Signal == syscall.SIGPIPE || exit.Code != 0), "the writer's error or the command's broken pipe: %v", err)
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
	t.Run("a command that did not start is done with its writers when Run returns", func(t *testing.T) {
		// The copies of the output pipes have ended: the caller may use
		// the writers at once, which -race checks.
		var stdout, stderr bytes.Buffer
		err := Run(t.Context(), Command{Class: JulietaGit, Argv: []string{"procgroup-test-no-such-command"}, Stdout: &stdout, Stderr: &stderr})
		require.ErrorIs(t, err, ErrNotFound)
		stdout.WriteString("the caller's")
		stderr.WriteString("the caller's")
		assert.Equal(t, "the caller's", stdout.String())
		assert.Equal(t, "the caller's", stderr.String())
	})
	t.Run("an absolute path that is not there is ErrNotFound", func(t *testing.T) {
		err := Run(t.Context(), Command{Class: JulietaGit, Argv: []string{filepath.Join(t.TempDir(), "missing")}})
		require.ErrorIs(t, err, ErrNotFound)
	})
	t.Run("a relative path that is not there in Dir is ErrNotFound", func(t *testing.T) {
		err := Run(t.Context(), Command{Class: JulietaGit, Argv: []string{"./missing"}, Dir: t.TempDir()})
		require.ErrorIs(t, err, ErrNotFound)
	})
	t.Run("a relative path is looked up in Dir, not in this process's directory", func(t *testing.T) {
		// procgroup.go is in this process's directory, not in Dir.
		require.FileExists(t, "procgroup.go")
		err := Run(t.Context(), Command{Class: JulietaGit, Argv: []string{"./procgroup.go"}, Dir: t.TempDir()})
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
// ends, and the drain after the kill ends once the pipes have been
// idle for WaitDelay, or when the run's context ends.
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
	t.Run("a cancellation with a stdin that never ends kills the grandchild within 1s", func(t *testing.T) {
		pidfile := filepath.Join(t.TempDir(), "pid")
		killAtEnd(t, pidfile)
		stdin, release := io.Pipe()
		t.Cleanup(func() { require.NoError(t, release.Close()) })
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		errc := make(chan error, 1)
		go func() {
			errc <- Run(ctx, Command{Class: Unbounded, Argv: []string{"/bin/sh", "-c", grandchildScript, pidfile}, Env: shellEnv, Stdin: stdin})
		}()
		pid := waitForPid(t, pidfile)
		start := time.Now()
		cancel()
		select {
		case err := <-errc:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(time.Second):
			require.FailNow(t, "Run waited on the stdin that never ends")
		}
		assert.Eventually(t, func() bool { return gone(pid) }, time.Second-time.Since(start), 5*time.Millisecond, "the grandchild %d is dead within 1s", pid)
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
	t.Run("a holder of the pipe outside the group does not hold a cancelled command", func(t *testing.T) {
		cmd, pidfile, _ := helper(t, "escape")
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
		cancel()
		require.ErrorIs(t, <-errc, context.Canceled)
		// The holder sleeps until the test ends it, so a command that
		// returned while it lives was not waiting for it.
		assert.NoError(t, syscall.Kill(holder, 0), "the holder is still alive, so it did not hold the command")
	})
	t.Run("a leader that exits 0 with a holder outside the group succeeds once the pipe is idle", func(t *testing.T) {
		cmd, pidfile, _ := helper(t, "escape-exit")
		start := time.Now()
		_, err := CombinedOutput(t.Context(), Command{Class: Unbounded, Argv: cmd.Args, Env: cmd.Env})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, time.Since(start), WaitDelay, "the drain waited for the idle pipe")
		// The holder sleeps until the test ends it, so a return while it
		// lives shows the idle bound, not the holder, ended the drain.
		assert.NoError(t, syscall.Kill(waitForPid(t, pidfile), 0), "the holder is still alive")
	})
	t.Run("a holder outside the group that never goes idle makes the run an error at its deadline", func(t *testing.T) {
		cmd, pidfile, _ := helper(t, "escape-chatty")
		// The deadline only ends the drain; what is checked is that a
		// drain cut short is an error and never a success.
		ctx, cancel := context.WithTimeout(t.Context(), 2*WaitDelay)
		defer cancel()
		_, err := CombinedOutput(ctx, Command{Class: Unbounded, Argv: cmd.Args, Env: cmd.Env})
		require.ErrorIs(t, err, context.DeadlineExceeded, "the output may be cut, so the run is not a success")
		assert.NoError(t, syscall.Kill(waitForPid(t, pidfile), 0), "the holder is still alive, so the drain did not end at its exit")
	})
	t.Run("a holder outside the group that never goes idle makes a run with no deadline an error at the cap", func(t *testing.T) {
		real := drainCap
		drainCap = 2 * WaitDelay
		t.Cleanup(func() { drainCap = real })
		cmd, pidfile, _ := helper(t, "escape-chatty")
		_, err := CombinedOutput(t.Context(), Command{Class: Unbounded, Argv: cmd.Args, Env: cmd.Env})
		require.ErrorIs(t, err, errDrainCap, "the output may be cut, so the run is not a success")
		assert.NoError(t, syscall.Kill(waitForPid(t, pidfile), 0), "the holder is still alive, so the drain did not end at its exit")
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

// openFiles returns how many files this process has open.
func openFiles(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/dev/fd")
	require.NoError(t, err)
	return len(entries)
}

// settles reports whether cond holds within 2s. It polls from the
// calling goroutine, since assert.Eventually runs cond on a goroutine of
// its own, which a count of goroutines would see.
func settles(cond func() bool) bool {
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if cond() {
			return true
		}
	}
	return cond()
}

// TestRunLeavesNothingBehind checks that Run, whether the command
// succeeds, fails or does not start, leaves no copy goroutine and no
// open file behind once its streams are done.
func TestRunLeavesNothingBehind(t *testing.T) {
	commands := map[string]Command{
		"a command that succeeds":   {Class: JulietaGit, Argv: []string{"/bin/sh", "-c", "cat; echo err >&2"}, Env: shellEnv},
		"a command that fails":      {Class: JulietaGit, Argv: []string{"/bin/sh", "-c", "exit 3"}, Env: shellEnv},
		"a command that never runs": {Class: JulietaGit, Argv: []string{"procgroup-test-no-such-command"}},
	}
	for name, c := range commands {
		t.Run(name, func(t *testing.T) {
			goroutines, files := runtime.NumGoroutine(), openFiles(t)
			var stdout, stderr bytes.Buffer
			c.Stdin, c.Stdout, c.Stderr = strings.NewReader("in\n"), &stdout, &stderr
			_ = Run(t.Context(), c)
			assert.True(t, settles(func() bool { return runtime.NumGoroutine() <= goroutines }), "no goroutine left")
			assert.True(t, settles(func() bool { return openFiles(t) <= files }), "no file left open")
		})
	}
}
