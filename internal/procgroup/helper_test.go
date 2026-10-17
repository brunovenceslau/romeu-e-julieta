// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The environment of a helper process: its role, and the files through
// which it tells the test its pids and that it is ready.
const (
	helperEnv  = "PROCGROUP_TEST_HELPER"
	pidfileEnv = "PROCGROUP_TEST_PIDFILE"
	readyEnv   = "PROCGROUP_TEST_READY"
	markerEnv  = "PROCGROUP_TEST_MARKER"
)

// helperLifetime bounds the life of a helper process.
const helperLifetime = 2 * time.Minute

// helperName is the name a helper gives Main, which the line of an
// interrupted run starts with.
const helperName = "helper"

// grandchildScript starts a sleep in the background, writes its pid to
// the file $0 names and waits for it: the sleep is a grandchild of
// whoever runs the script, in the same process group, and holds the
// script's standard output and standard error.
const grandchildScript = `sleep 60 & echo $! > "$0"; wait`

// shellEnv is the environment of the shell commands of the tests.
var shellEnv = []string{"PATH=/usr/bin:/bin"}

// TestHelperProcess is not a test: the tests of this package run this
// binary again with helperEnv set, to get a process with the behavior
// they need.
func TestHelperProcess(t *testing.T) {
	role := os.Getenv(helperEnv)
	if role == "" {
		return
	}
	// A helper whose test died does not outlive it by long.
	time.AfterFunc(helperLifetime, func() { os.Exit(3) })
	switch role {
	case "escape", "escape-exit", "escape-chatty":
		// Hold the output pipe from a session of its own, which a kill of
		// the process group does not reach, and write its pid; then the
		// leader waits, or with escape-exit and escape-chatty exits 0 at
		// once. The holder of escape-chatty writes without a pause.
		holderRole := "sleep"
		if role == "escape-chatty" {
			holderRole = "chatter"
		}
		child := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
		child.Env = append(os.Environ(), helperEnv+"="+holderRole)
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		writeOrExit(os.Getenv(pidfileEnv), strconv.Itoa(child.Process.Pid))
		if role == "escape" {
			time.Sleep(time.Minute)
		}
	case "sleep":
		// The holder outlives every wait of the tests; killAtEnd ends it.
		time.Sleep(helperLifetime)
	case "chatter":
		// A holder whose output never goes idle; killAtEnd ends it. Once
		// the run returns and closes the pipe, it stops writing but lives
		// on, so a test that finds it alive after the run is not racing
		// its exit. A write to a broken standard output would kill it by
		// SIGPIPE, so that signal is ignored.
		signal.Ignore(syscall.SIGPIPE)
		for {
			if _, err := os.Stdout.WriteString("x\n"); err != nil {
				time.Sleep(helperLifetime)
			}
			time.Sleep(10 * time.Millisecond)
		}
	case "group":
		// A command whose group holds a grandchild when the signal lands.
		os.Exit(Main(helperName, os.Stderr, func(ctx context.Context) int {
			err := Run(ctx, Command{
				Class: Unbounded,
				Argv:  []string{"/bin/sh", "-c", grandchildScript, os.Getenv(pidfileEnv)},
				Env:   shellEnv,
			})
			if err != nil {
				return 1
			}
			return 0
		}))
	case "failed", "finished":
		// A command that found a failure (2) or finished its work (0)
		// and returns once the signal has cancelled it.
		status := 2
		if role == "finished" {
			status = 0
		}
		os.Exit(Main(helperName, os.Stderr, func(ctx context.Context) int {
			writeOrExit(os.Getenv(readyEnv), "ready")
			<-ctx.Done()
			return status
		}))
	case "absorb", "stepped":
		// A command that returns 2 once cancelled, with the handler held
		// right before it steps aside: absorb sends this process a second
		// signal there; stepped writes the marker file after a pause, so
		// the test sees whether Main waited for the handler.
		beforeStepAside = func() {
			if role == "absorb" {
				_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
			}
			time.Sleep(200 * time.Millisecond)
			writeOrExit(os.Getenv(markerEnv), "stepped")
		}
		os.Exit(Main(helperName, os.Stderr, func(ctx context.Context) int {
			writeOrExit(os.Getenv(readyEnv), "ready")
			<-ctx.Done()
			return 2
		}))
	case "deaf":
		// A command that does not return after the first signal, so only
		// the second one can end the process.
		os.Exit(Main(helperName, os.Stderr, func(_ context.Context) int {
			writeOrExit(os.Getenv(readyEnv), "ready")
			time.Sleep(30 * time.Second)
			return 0
		}))
	}
	os.Exit(0)
}

// writeOrExit writes the file or ends the helper with status 2.
func writeOrExit(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		os.Exit(2)
	}
}

// helper returns the command that runs this test binary as the helper
// of the role, with the pid file and the ready file in a directory of
// the test, and the marker file beside them.
func helper(t *testing.T, role string) (cmd *exec.Cmd, pidfile, ready string) {
	t.Helper()
	dir := t.TempDir()
	pidfile = filepath.Join(dir, "pid")
	ready = filepath.Join(dir, "ready")
	killAtEnd(t, pidfile)
	cmd = exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHelperProcess$")
	cmd.Env = append(os.Environ(), helperEnv+"="+role, pidfileEnv+"="+pidfile, readyEnv+"="+ready, markerEnv+"="+filepath.Join(dir, "marker"))
	return cmd, pidfile, ready
}

// waitForFile waits until the file exists.
func waitForFile(t *testing.T, path string) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := os.Stat(path)
		return err == nil
	}, 10*time.Second, 10*time.Millisecond, "the helper writes %s", path)
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

// gone reports whether the process is dead: no such pid, or, on Linux,
// a zombie that /proc shows in state Z, as pid 1 of a container may
// not reap. On macOS launchd reaps an orphan, so the pid goes away.
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

// assertGone waits until the process is dead.
func assertGone(t *testing.T, pid int) {
	t.Helper()
	assert.Eventually(t, func() bool { return gone(pid) }, 5*time.Second, 10*time.Millisecond, "the process %d is dead", pid)
}
