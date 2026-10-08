// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helperEnv names the role of this test binary when a test runs it as a
// helper process; an empty value means it is the test run itself.
const helperEnv = "CI_TEST_HELPER"

// TestHelperProcess is not a test: the tests below run this binary
// again with helperEnv set, to get processes with the behavior they
// need.
func TestHelperProcess(t *testing.T) {
	switch os.Getenv(helperEnv) {
	case "":
		return
	case "escape":
		// Hold the output pipe from a session of its own, which a kill of
		// the process group does not reach, and write its pid.
		child := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
		child.Env = append(os.Environ(), helperEnv+"=sleep")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		if err := os.WriteFile(os.Getenv("CI_TEST_PIDFILE"), []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
			os.Exit(2)
		}
		time.Sleep(time.Minute)
	case "sleep":
		time.Sleep(15 * time.Second)
	case "main":
		// The real signal handling of main, around a step that sleeps.
		e := env{dir: os.Getenv("CI_TEST_DIR"), stdin: strings.NewReader(""), stdout: os.Stdout, stderr: os.Stderr, steps: []step{groupStep(os.Getenv("CI_TEST_PIDFILE"))}}
		os.Exit(runSignalled(e, []string{"fast"}))
	}
	os.Exit(0)
}

// groupStep is a step whose command forks a grandchild that ignores
// SIGTERM, writes its pid to pidfile and waits on it: only a SIGKILL
// to the whole process group ends it.
func groupStep(pidfile string) step {
	script := `trap "" TERM; sleep 60 & echo $! > "$0.tmp" && mv "$0.tmp" "$0"; wait`
	return step{name: "group", argv: []string{"sh", "-c", script, pidfile}, environ: []string{"PATH=" + os.Getenv("PATH")}}
}

// waitForPid returns the pid that the file holds, once it is there.
func waitForPid(t *testing.T, pidfile string) int {
	t.Helper()
	var pid int
	require.Eventually(t, func() bool {
		b, err := os.ReadFile(pidfile)
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(strings.TrimSpace(string(b)))
		return err == nil
	}, 10*time.Second, 10*time.Millisecond, "the grandchild writes its pid")
	return pid
}

// requireGone fails unless the process is dead, once init has reaped
// it. Signal 0 only checks that the pid exists.
func requireGone(t *testing.T, pid int) {
	t.Helper()
	require.Eventually(t, func() bool { return syscall.Kill(pid, 0) == syscall.ESRCH },
		5*time.Second, 10*time.Millisecond, "process %d is dead", pid)
}

// killAtEnd makes sure a process of a failing test does not outlive it.
func killAtEnd(t *testing.T, pidfile string) {
	t.Helper()
	t.Cleanup(func() {
		if b, err := os.ReadFile(pidfile); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
}

// TestKillWithGroup kills the whole group, with the SIGKILL that a
// grandchild cannot ignore (the group mutants: no Setpgid, the pid and
// not the group, SIGTERM, no Cancel), and lets the wait end when a
// holder of the pipe has left the group (the WaitDelay mutants).
func TestKillWithGroup(t *testing.T) {
	t.Run("the grandchild that ignores SIGTERM is killed with the group", func(t *testing.T) {
		pidfile := filepath.Join(t.TempDir(), "pid")
		killAtEnd(t, pidfile)
		s := groupStep(pidfile)
		s.timeout = 500 * time.Millisecond
		start := time.Now()
		_, err := s.run(t.Context(), t.TempDir())
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, time.Since(start), 5*time.Second)
		requireGone(t, waitForPid(t, pidfile))
	})
	t.Run("a holder of the pipe outside the group is waited for no longer than WaitDelay", func(t *testing.T) {
		pidfile := filepath.Join(t.TempDir(), "pid")
		killAtEnd(t, pidfile)
		const bound = 6 * time.Second // the holder sleeps for 15s
		require.Less(t, killWaitDelay, bound-time.Second, "the bound of the wait is under the bound of the test")
		s := step{name: "escape", argv: []string{os.Args[0], "-test.run=^TestHelperProcess$"}, timeout: 200 * time.Millisecond,
			environ: []string{"PATH=" + os.Getenv("PATH"), helperEnv + "=escape", "CI_TEST_PIDFILE=" + pidfile}}
		start := time.Now()
		_, err := s.run(t.Context(), t.TempDir())
		require.Error(t, err)
		assert.Less(t, time.Since(start), bound, "the step returns after WaitDelay, not when the holder ends")
	})
}

// TestKillGroupAfterWait checks that a group that is already gone is
// os.ErrProcessDone, which Go does not report, and not a kill error
// that would turn a command that succeeded into a failure.
func TestKillGroupAfterWait(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "true")
	killWithGroup(cmd)
	require.NoError(t, cmd.Run())
	assert.ErrorIs(t, killGroup(cmd.Process.Pid), os.ErrProcessDone)
}

// TestMainSignals runs main's signal handling around a step that
// sleeps, sends the signal, and expects a prompt exit with the whole
// group of the step killed, for each of SIGINT and SIGTERM.
func TestMainSignals(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			r := newTree(t)
			r.Commit("fixture")
			pidfile := filepath.Join(t.TempDir(), "pid")
			killAtEnd(t, pidfile)
			cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
			cmd.Env = append(r.Env, "PATH="+os.Getenv("PATH"), helperEnv+"=main", "CI_TEST_DIR="+r.Dir, "CI_TEST_PIDFILE="+pidfile)
			require.NoError(t, cmd.Start())
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			t.Cleanup(func() { _ = cmd.Process.Kill() })
			grandchild := waitForPid(t, pidfile)
			// The signal handler is installed before any step runs.
			require.NoError(t, cmd.Process.Signal(sig))
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("main did not exit after the signal")
			}
			requireGone(t, grandchild)
		})
	}
}
