// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

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
const helperEnv = "PROCGROUP_TEST_HELPER"

// TestHelperProcess is not a test: the tests below run this binary
// again with helperEnv set, to get a process with the behavior they
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
		if err := os.WriteFile(os.Getenv("PROCGROUP_TEST_PIDFILE"), []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
			os.Exit(2)
		}
		time.Sleep(time.Minute)
	case "sleep":
		time.Sleep(15 * time.Second)
	}
	os.Exit(0)
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

// killAtEnd makes sure the process whose pid the file holds does not
// outlive a failing test. A file that was never written is skipped.
func killAtEnd(t *testing.T, pidfile string) {
	t.Helper()
	t.Cleanup(func() {
		b, err := os.ReadFile(pidfile)
		if err != nil {
			return
		}
		// Pids 0 and 1 are never ours to kill: -1 would reach every process.
		if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && pid > 1 {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
}

// gone reports whether the process is dead: no such pid, or a zombie
// that /proc shows in state Z, as pid 1 of a container may not reap.
func gone(pid int) bool {
	if syscall.Kill(pid, 0) == syscall.ESRCH {
		return true
	}
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	i := strings.LastIndex(string(b), ") ")
	return i >= 0 && i+2 < len(b) && b[i+2] == 'Z'
}

// TestCombinedOutputKillsTheGroupAfterTheLeader checks that a
// grandchild that outlives a leader which ended at once is killed when
// CombinedOutput returns, and that the output and the status are the
// leader's.
func TestCombinedOutputKillsTheGroupAfterTheLeader(t *testing.T) {
	pidfile := filepath.Join(t.TempDir(), "pid")
	script := `sleep 60 >/dev/null 2>&1 & echo $! > "$0"; echo out; exit 3`
	cmd := exec.CommandContext(t.Context(), "sh", "-c", script, pidfile)
	out, err := CombinedOutput(cmd)
	var exit *exec.ExitError
	require.ErrorAs(t, err, &exit)
	assert.Equal(t, 3, exit.ExitCode())
	assert.Equal(t, "out\n", string(out))
	b, err := os.ReadFile(pidfile)
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	assert.Eventually(t, func() bool { return gone(pid) }, 5*time.Second, 10*time.Millisecond, "the grandchild %d is dead", pid)
}

// TestKillWithGroup kills the whole group, with the SIGKILL that a
// grandchild cannot ignore (the group mutants: no Setpgid, the pid and
// not the group, SIGTERM, no Cancel), and lets the wait end when a
// holder of the pipe has left the group (the WaitDelay mutants).
func TestKillWithGroup(t *testing.T) {
	t.Run("the grandchild that ignores SIGTERM is killed with the group", func(t *testing.T) {
		pidfile := filepath.Join(t.TempDir(), "pid")
		killAtEnd(t, pidfile)
		script := `trap "" TERM; sleep 60 & echo $! > "$0"; wait`
		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()
		cmd := exec.CommandContext(ctx, "sh", "-c", script, pidfile)
		KillWithGroup(cmd)
		start := time.Now()
		_, err := cmd.CombinedOutput()
		require.Error(t, err)
		require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
		assert.Less(t, time.Since(start), 5*time.Second)
		pid := waitForPid(t, pidfile)
		assert.Eventually(t, func() bool { return gone(pid) }, 5*time.Second, 10*time.Millisecond, "the grandchild %d is dead", pid)
	})
	t.Run("a holder of the pipe outside the group is waited for no longer than WaitDelay", func(t *testing.T) {
		pidfile := filepath.Join(t.TempDir(), "pid")
		killAtEnd(t, pidfile)
		const bound = 6 * time.Second // the holder sleeps for 15s
		require.Less(t, WaitDelay, bound-time.Second, "the bound of the wait is under the bound of the test")
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHelperProcess$")
		cmd.Env = append(os.Environ(), helperEnv+"=escape", "PROCGROUP_TEST_PIDFILE="+pidfile)
		KillWithGroup(cmd)
		errc := make(chan error, 1)
		go func() {
			_, err := cmd.CombinedOutput()
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

// TestKillGroupAfterWait checks that a group that is already gone is
// os.ErrProcessDone, which Go does not report, and not a kill error
// that would turn a command that succeeded into a failure.
func TestKillGroupAfterWait(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "true")
	KillWithGroup(cmd)
	require.NoError(t, cmd.Run())
	assert.ErrorIs(t, KillGroup(cmd.Process.Pid), os.ErrProcessDone)
}
