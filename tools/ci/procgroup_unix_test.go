// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package main

import (
	"bytes"
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
		if err := os.WriteFile(os.Getenv("CI_TEST_PIDFILE")+leaderSuffix, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			os.Exit(2)
		}
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
		// The real signal handling of main, around a step that sleeps, or
		// with CI_TEST_HOLDER set, around one whose holder of the output
		// pipe makes the drain after a kill last for WaitDelay, or with
		// CI_TEST_FAILFAST set, around one that fails at once and leaves a
		// grandchild on the pipe.
		pidfile := os.Getenv("CI_TEST_PIDFILE")
		st := groupStep(pidfile)
		if os.Getenv("CI_TEST_FAILFAST") != "" {
			st = failFastStep(pidfile)
		}
		if os.Getenv("CI_TEST_HOLDER") != "" {
			st = step{name: "holder", argv: []string{os.Args[0], "-test.run=^TestHelperProcess$"},
				environ: []string{"PATH=" + os.Getenv("PATH"), helperEnv + "=escape", "CI_TEST_PIDFILE=" + pidfile}}
		}
		e := env{dir: os.Getenv("CI_TEST_DIR"), stdin: strings.NewReader(""), stdout: os.Stdout, stderr: os.Stderr, steps: []step{st}}
		os.Exit(runSignalled(e, []string{"fast"}))
	}
	os.Exit(0)
}

// leaderSuffix names the file next to a pidfile that holds the pid of
// the leader of the step's process group, written before anything else
// so that a test which fails early can still end the whole group.
const leaderSuffix = ".leader"

// groupStep is a step whose command forks a grandchild that ignores
// SIGTERM, writes its pid to pidfile and waits on it: only a SIGKILL
// to the whole process group ends it.
func groupStep(pidfile string) step {
	script := `echo $$ > "$0.leader"; trap "" TERM; sleep 60 & echo $! > "$0.tmp" && mv "$0.tmp" "$0"; wait`
	return step{name: "group", argv: []string{"sh", "-c", script, pidfile}, environ: []string{"PATH=" + os.Getenv("PATH")}}
}

// failFastStep is a step that writes the pid of a grandchild, which
// holds the output pipe, and then fails at once: the step has a failing
// status while its run goes on until the group is killed.
func failFastStep(pidfile string) step {
	script := `echo $$ > "$0.leader"; sleep 60 & echo $! > "$0.tmp" && mv "$0.tmp" "$0"; exit 1`
	return step{name: "failfast", argv: []string{"sh", "-c", script, pidfile}, environ: []string{"PATH=" + os.Getenv("PATH")}}
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

// requireGone fails unless the process is dead. Signal 0 only checks
// that the pid exists, and a zombie still does when pid 1 does not
// reap, so on linux the state Z of /proc/<pid>/stat counts as dead too;
// elsewhere there is no /proc and the pid must be gone.
func requireGone(t *testing.T, pid int) {
	t.Helper()
	require.Eventually(t, func() bool { return syscall.Kill(pid, 0) == syscall.ESRCH || isZombie(pid) },
		5*time.Second, 10*time.Millisecond, "process %d is dead", pid)
}

// isZombie reports whether /proc says the process has ended and waits
// to be reaped. The state follows the last ")" of the line, because the
// name before it may hold anything.
func isZombie(pid int) bool {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	i := strings.LastIndex(string(b), ") ")
	return i >= 0 && i+2 < len(b) && b[i+2] == 'Z'
}

// killAtEnd makes sure the processes of a failing test do not outlive
// it: whatever pidfile and its leader file hold is killed with its whole
// process group. A file that was never written, or is half written,
// is skipped, and the other one still counts.
func killAtEnd(t *testing.T, pidfile string) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{pidfile + leaderSuffix, pidfile} {
			b, err := os.ReadFile(name)
			if err != nil {
				continue
			}
			// Pids 0 and 1 are never ours to kill: -1 would reach every process.
			if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && pid > 1 {
				// The group goes only if pid still leads one: a pid that was
				// recycled after the process ended does not.
				if pgid, err := syscall.Getpgid(pid); err == nil && pgid == pid {
					_ = syscall.Kill(-pid, syscall.SIGKILL)
				}
				// The same for the process alone, which may not lead a group.
				if pid != os.Getpid() {
					if _, err := syscall.Getpgid(pid); err == nil {
						_ = syscall.Kill(pid, syscall.SIGKILL)
					}
				}
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
		s := step{name: "escape", argv: []string{os.Args[0], "-test.run=^TestHelperProcess$"},
			environ: []string{"PATH=" + os.Getenv("PATH"), helperEnv + "=escape", "CI_TEST_PIDFILE=" + pidfile}}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		errc := make(chan error, 1)
		go func() {
			_, err := s.run(ctx, t.TempDir())
			errc <- err
		}()
		// The holder exists before the step is cancelled, so the wait that
		// follows is for a live holder and not for nothing.
		holder := waitForPid(t, pidfile)
		start := time.Now()
		cancel()
		require.Error(t, <-errc)
		assert.Less(t, time.Since(start), bound, "the step returns after WaitDelay, not when the holder ends")
		assert.NoError(t, syscall.Kill(holder, 0), "the holder is still alive, so only WaitDelay ended the wait")
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
// sleeps, sends the signal, and expects a prompt exit with status 130
// or 143 and the whole group of the step killed, for each of SIGINT and
// SIGTERM. The sibling case has a step that fails fast and keeps running.
func TestMainSignals(t *testing.T) {
	for _, c := range []struct {
		sig  syscall.Signal
		code int
	}{{syscall.SIGINT, 130}, {syscall.SIGTERM, 143}} {
		sig := c.sig
		t.Run(sig.String(), func(t *testing.T) {
			r := newTree(t)
			r.Commit("fixture")
			pidfile := filepath.Join(t.TempDir(), "pid")
			killAtEnd(t, pidfile)
			cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
			cmd.Env = append(r.Env, "PATH="+os.Getenv("PATH"), helperEnv+"=main", "CI_TEST_DIR="+r.Dir, "CI_TEST_PIDFILE="+pidfile)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			require.NoError(t, cmd.Start())
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			t.Cleanup(func() { _ = cmd.Process.Kill() })
			grandchild := waitForPid(t, pidfile)
			// The signal handler is installed before any step runs.
			require.NoError(t, cmd.Process.Signal(sig))
			select {
			case err := <-done:
				var exit *exec.ExitError
				require.ErrorAs(t, err, &exit, "the run ends with a status")
				assert.Equal(t, c.code, exit.ExitCode())
				assert.Equal(t, 1, strings.Count(stderr.String(), "ci: interrupted\n"), "one line says the run was interrupted")
			case <-time.After(10 * time.Second):
				t.Fatal("main did not exit after the signal")
			}
			requireGone(t, grandchild)
		})
	}
}

// TestMainSignalAfterFailure runs main around a step that has already
// failed while a grandchild still holds its pipe, signals it with SIGINT,
// and expects status 130 and one interrupted line. The grandchild is not
// checked: the leader of the group has ended, and a measured run left it
// alive after the signal (a pending item), so killAtEnd ends it.
func TestMainSignalAfterFailure(t *testing.T) {
	r := newTree(t)
	r.Commit("fixture")
	pidfile := filepath.Join(t.TempDir(), "pid")
	killAtEnd(t, pidfile)
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	cmd.Env = append(r.Env, "PATH="+os.Getenv("PATH"), helperEnv+"=main", "CI_TEST_FAILFAST=1", "CI_TEST_DIR="+r.Dir, "CI_TEST_PIDFILE="+pidfile)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	waitForPid(t, pidfile)
	require.NoError(t, cmd.Process.Signal(syscall.SIGINT))
	select {
	case err := <-done:
		var exit *exec.ExitError
		require.ErrorAs(t, err, &exit, "the run ends with a status")
		assert.Equal(t, 130, exit.ExitCode())
		assert.Equal(t, 1, strings.Count(stderr.String(), "ci: interrupted\n"), "one line says the run was interrupted")
	case <-time.After(10 * time.Second):
		t.Fatal("main did not exit after the signal")
	}
}

// TestFinish holds the four cells of the exit status of a run: with and
// without a signal, with a passing and a failing status.
func TestFinish(t *testing.T) {
	const line = "ci: interrupted\n"
	for _, c := range []struct {
		name   string
		status int
		sig    syscall.Signal
		want   int
		out    string
	}{
		{"a pass that no signal touched stays 0", exitOK, 0, 0, ""},
		{"a pass that a signal followed stays 0", exitOK, syscall.SIGINT, 0, ""},
		{"a failure that no signal touched keeps its status", exitFail, 0, 1, ""},
		{"a refusal that no signal touched keeps its status", exitError, 0, 2, ""},
		{"SIGINT during a run gives 130 and one line", exitFail, syscall.SIGINT, 130, line},
		{"SIGTERM during a run gives 143 and one line", exitFail, syscall.SIGTERM, 143, line},
		{"the signal wins over a refusal", exitError, syscall.SIGINT, 130, line},
		{"SIGTERM wins over a refusal and gives 143", exitError, syscall.SIGTERM, 143, line},
		{"a failure with SIGTERM gives 143", exitFail, syscall.SIGTERM, 143, line},
		{"a pass that SIGTERM followed stays 0", exitOK, syscall.SIGTERM, 0, ""},
		{"128 plus the number holds for a signal other than INT or TERM", exitFail, syscall.SIGHUP, 129, line},
	} {
		t.Run(c.name, func(t *testing.T) {
			var out bytes.Buffer
			assert.Equal(t, c.want, finish(&out, c.status, c.sig))
			assert.Equal(t, c.out, out.String())
		})
	}
}

// TestMainSecondSignal checks that the handler steps aside after the
// first signal: a second one ends main while it still drains the output
// pipe of a step whose holder escaped the group, and does not wait for
// WaitDelay.
func TestMainSecondSignal(t *testing.T) {
	r := newTree(t)
	r.Commit("fixture")
	pidfile := filepath.Join(t.TempDir(), "pid")
	killAtEnd(t, pidfile)
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	cmd.Env = append(r.Env, "PATH="+os.Getenv("PATH"), helperEnv+"=main", "CI_TEST_HOLDER=1", "CI_TEST_DIR="+r.Dir, "CI_TEST_PIDFILE="+pidfile)
	require.NoError(t, cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	waitForPid(t, pidfile)
	start := time.Now()
	// SIGTERM, not SIGINT: a test run in the background may inherit SIGINT
	// as ignored, and Stop would restore that, so the repeats would be dropped.
	require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
	// Signals repeat until the process ends: one that lands before the
	// handler stepped aside is swallowed, and the next one is not.
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-done:
			var exit *exec.ExitError
			require.ErrorAs(t, err, &exit)
			ws, ok := exit.Sys().(syscall.WaitStatus)
			require.True(t, ok)
			assert.True(t, ws.Signaled(), "the second signal ended the process by its default action")
			assert.Less(t, time.Since(start), killWaitDelay-250*time.Millisecond, "main did not wait out the drain")
			return
		case <-tick.C:
			_ = cmd.Process.Signal(syscall.SIGTERM)
		case <-time.After(10 * time.Second):
			t.Fatal("main did not exit after the second signal")
		}
	}
}
