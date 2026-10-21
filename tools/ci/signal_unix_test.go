// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"slices"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// helperEnv names the role of this test binary when a test runs it as a
// helper process; an empty value means it is the test run itself.
const helperEnv = "CI_TEST_HELPER"

// raceEnv is the entry that a test adds to the environment of a helper
// process, which is this test binary run again. A program built with the
// race detector sleeps before it exits for the atexit_sleep_ms option of
// GORACE, 1000 ms by default, and a helper whose environment is built from
// nothing does not set it: each helper would take a second to exit. 0 ends
// the sleep, and the rest of the environment is the test's own. The module
// holds one such constant in each package that starts a helper, because
// the imports check (REQ-0971, REQ-0972) refuses a shared package below
// internal/ for tools/** and for a package no row of 10 10.7 names.
const raceEnv = "GORACE=atexit_sleep_ms=0"

// TestHelperProcess is not a test: the tests below run this binary
// again with helperEnv set, to get processes with the behavior they
// need.
func TestHelperProcess(t *testing.T) {
	switch os.Getenv(helperEnv) {
	case "":
		return
	case "main":
		// The real signal handling of main, around a step that sleeps, and
		// with CI_TEST_FAILFAST set, after a step that has already
		// failed. With CI_TEST_SECOND_SIGNAL set, the handler sends this
		// process a second SIGTERM right before it steps aside and waits
		// until it reaches the handler's channel. It writes its own pid
		// first, for the test to signal.
		if os.Getenv("CI_TEST_SECOND_SIGNAL") != "" {
			procgroup.SetBeforeStepAside(func(sigs <-chan os.Signal) {
				_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
				select {
				case <-sigs:
				case <-time.After(10 * time.Second):
					os.Exit(4) // the signal never reached the handler
				}
			})
		}
		pidfile := os.Getenv("CI_TEST_PIDFILE")
		if err := writePid(pidfile+mainSuffix, os.Getpid()); err != nil {
			os.Exit(2)
		}
		steps := []check.Step{groupStep(pidfile)}
		if os.Getenv("CI_TEST_FAILFAST") != "" {
			failed := check.Step{Name: "failed", Argv: []string{"sh", "-c", "exit 1"}, Environ: []string{"PATH=" + os.Getenv("PATH")}}
			steps = append([]check.Step{failed}, steps...)
		}
		e := check.Env{Dir: os.Getenv("CI_TEST_DIR"), Stdin: strings.NewReader(""), Stdout: os.Stdout, Stderr: os.Stderr, Steps: steps}
		os.Exit(runMain(e, []string{"fast"}))
	}
	os.Exit(0)
}

// The files next to a pidfile: the pid of the leader of the step's
// process group, written before anything else so that a test which
// fails early can still end the whole group, and the pid of the helper
// that runs main.
const (
	leaderSuffix = ".leader"
	mainSuffix   = ".main"
)

// writePid writes pid to the file through a temporary file and a
// rename, so a reader never sees half of it.
func writePid(file string, pid int) error {
	if err := os.WriteFile(file+".tmp", []byte(strconv.Itoa(pid)), 0o600); err != nil {
		return err
	}
	return os.Rename(file+".tmp", file)
}

// groupStep is a step whose command forks a grandchild that ignores
// SIGTERM, writes its pid to pidfile and waits on it: only a SIGKILL
// to the whole process group ends it.
func groupStep(pidfile string) check.Step {
	script := `echo $$ > "$0.leader"; trap "" TERM; sleep 60 & echo $! > "$0.tmp" && mv "$0.tmp" "$0"; wait`
	return check.Step{Name: "group", Argv: []string{"sh", "-c", script, pidfile}, Environ: []string{"PATH=" + os.Getenv("PATH")}}
}

// leaderExitStep is a step that writes the pid of a grandchild, which
// holds the output pipe, and then ends at once with status code: the
// step has its status while its group still runs, until the group is
// killed.
func leaderExitStep(pidfile string, code int) check.Step {
	script := `echo $$ > "$0.leader"; sleep 60 & echo $! > "$0.tmp" && mv "$0.tmp" "$0"; exit ` + strconv.Itoa(code)
	return check.Step{Name: "leaderexit", Argv: []string{"sh", "-c", script, pidfile}, Environ: []string{"PATH=" + os.Getenv("PATH")}}
}

// waitForPid returns the pid that the file holds, once it is there. A
// pid at or below 1 is never one a test may signal (kill(-1) reaches
// every process, kill(0) the test's own group), so it fails the test.
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
	}, 10*time.Second, 10*time.Millisecond, "the process writes its pid")
	require.Greater(t, pid, 1, "the pid in %s", pidfile)
	return pid
}

// runAsync runs the command through procgroup.Run in a goroutine of its
// own and returns the channel that gets its result.
func runAsync(ctx context.Context, c procgroup.Command) <-chan error {
	done := make(chan error, 1)
	go func() { done <- procgroup.Run(ctx, c) }()
	return done
}

// startMain runs this binary as the helper that runs main over a
// fixture tree, with the extra environment, and returns its result
// channel, its standard error, its pid and the pid of the step's
// grandchild. The helper runs in a process group of its own, under a
// limit that ends a helper the test lost.
func startMain(t *testing.T, extra ...string) (done <-chan error, stderr *bytes.Buffer, pid, grandchild int) {
	t.Helper()
	r := citest.NewTree(t)
	r.Commit("fixture")
	pidfile := filepath.Join(t.TempDir(), "pid")
	killAtEnd(t, pidfile)
	stderr = &bytes.Buffer{}
	ctx := procgroup.WithTestTimeout(t.Context(), procgroup.ToolStep, 30*time.Second)
	done = runAsync(ctx, procgroup.Command{
		Class:  procgroup.ToolStep,
		Argv:   []string{os.Args[0], "-test.run=^TestHelperProcess$"},
		Env:    append(append(slices.Clone(r.Env), raceEnv, "PATH="+os.Getenv("PATH"), helperEnv+"=main", "CI_TEST_DIR="+r.Dir, "CI_TEST_PIDFILE="+pidfile), extra...),
		Stderr: stderr,
	})
	pid = waitForPid(t, pidfile+mainSuffix)
	// The step runs, and so the signal handler is installed, once the
	// grandchild has written its pid.
	grandchild = waitForPid(t, pidfile)
	return done, stderr, pid, grandchild
}

// exitOf waits for the result of a helper and returns how it exited.
func exitOf(t *testing.T, done <-chan error) *procgroup.ExitError {
	t.Helper()
	select {
	case err := <-done:
		exit, ok := errors.AsType[*procgroup.ExitError](err)
		require.True(t, ok, "the run ends with a status: %v", err)
		return exit
	case <-time.After(10 * time.Second):
		require.FailNow(t, "main did not exit after the signal")
	}
	return nil
}

// requireGone fails unless the process is dead. Signal 0 only checks
// that the pid exists, and a zombie still does when pid 1 does not
// reap, so on linux the state Z of /proc/<pid>/stat counts as dead too;
// elsewhere there is no /proc and the pid must be gone.
func requireGone(t *testing.T, pid int) {
	t.Helper()
	require.Greater(t, pid, 1, "a pid a test may signal")
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
// it: whatever pidfile and the files next to it hold is killed with its
// whole process group. A file that was never written, or is half
// written, is skipped, and the others still count.
func killAtEnd(t *testing.T, pidfile string) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{pidfile + mainSuffix, pidfile + leaderSuffix, pidfile} {
			b, err := os.ReadFile(name)
			if err != nil {
				continue
			}
			// Pids 0 and 1 are never ours to kill: -1 would reach every process.
			if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && pid > 1 {
				// The group goes only if pid exists and leads one other than
				// this test's own: this check cannot tell a recycled pid that
				// leads a group from the original, which is the most a cleanup
				// of a failed test asks.
				if pgid, err := syscall.Getpgid(pid); err == nil && pgid == pid && pid != syscall.Getpgrp() {
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

// TestStepRunKillsTheGroup checks that a step whose leader has ended,
// by failing or by succeeding, leaves no grandchild behind once run
// returns: the grandchild holds the output pipe, and procgroup kills
// the group as soon as the leader ends, so the pipe closes and the
// step's result is the leader's. That the kill comes before the drain,
// so the drain bound is not waited out, is procgroup's to show
// (TestRunKillsTheGroup there); the bound here only catches a step
// that waits for the grandchild, which sleeps 60 s, and leaves the
// slack a loaded machine needs.
func TestStepRunKillsTheGroup(t *testing.T) {
	for _, c := range []struct {
		name string
		code int
	}{{"a leader that fails at once", 1}, {"a leader that succeeds at once", 0}} {
		t.Run(c.name, func(t *testing.T) {
			pidfile := filepath.Join(t.TempDir(), "pid")
			killAtEnd(t, pidfile)
			start := time.Now()
			_, err := leaderExitStep(pidfile, c.code).Run(t.Context(), t.TempDir())
			took := time.Since(start)
			if c.code == 0 {
				require.NoError(t, err, "the leader succeeded, and its group is gone")
			} else {
				exit, ok := errors.AsType[*procgroup.ExitError](err)
				require.True(t, ok, "the leader's status: %v", err)
				assert.Equal(t, c.code, exit.Code)
			}
			requireGone(t, waitForPid(t, pidfile))
			assert.Less(t, took, procgroup.WaitDelay+20*time.Second, "the run did not wait for the grandchild")
		})
	}
}

// TestMainSignals runs main's signal handling around a step that
// sleeps, sends the signal, and expects a prompt exit with status 130
// or 143 and the whole group of the step killed, for each of SIGINT and
// SIGTERM.
func TestMainSignals(t *testing.T) {
	for _, c := range []struct {
		sig  syscall.Signal
		code int
	}{{syscall.SIGINT, 130}, {syscall.SIGTERM, 143}} {
		t.Run(c.sig.String(), func(t *testing.T) {
			done, stderr, pid, grandchild := startMain(t)
			require.NoError(t, syscall.Kill(pid, c.sig))
			exit := exitOf(t, done)
			assert.Equal(t, c.code, exit.Code)
			assert.Equal(t, 1, strings.Count(stderr.String(), "ci: interrupted\n"), "one line says the run was interrupted")
			requireGone(t, grandchild)
		})
	}
}

// TestMainSignalAfterFailure runs main while a step's group runs after
// an earlier step has already failed, signals it with SIGINT, and
// expects status 130, one interrupted line and the grandchild gone:
// the signal wins over the failure.
func TestMainSignalAfterFailure(t *testing.T) {
	done, stderr, pid, grandchild := startMain(t, "CI_TEST_FAILFAST=1")
	require.NoError(t, syscall.Kill(pid, syscall.SIGINT))
	exit := exitOf(t, done)
	assert.Equal(t, 130, exit.Code)
	assert.Equal(t, 1, strings.Count(stderr.String(), "ci: interrupted\n"), "one line says the run was interrupted")
	requireGone(t, grandchild)
}

// TestMainSecondSignalAbsorbed is REQ-1234 for tools/ci: a run that
// gets two SIGTERMs while a step's group runs exits 143, as for the
// first signal alone, with the group gone and the second signal
// absorbed rather than ending the process by its default action.
//
// The test sends the first SIGTERM; the helper's handler sends the
// second to its own process right before it steps aside, after the
// cancel and the kill of every group, and reads it from the handler's
// channel before the handler steps aside. A signal a process sends
// itself may arrive after kill returns, and after the handler's Stop
// it would end the process, so the read is what puts the second signal
// inside the window whatever the scheduling. Two signals sent back to
// back by the test raced the handler instead, and a test held between
// them failed (the second ended the process).
func TestMainSecondSignalAbsorbed(t *testing.T) {
	done, stderr, pid, grandchild := startMain(t, "CI_TEST_SECOND_SIGNAL=1")
	require.NoError(t, syscall.Kill(pid, syscall.SIGTERM))
	exit := exitOf(t, done)
	assert.Zero(t, exit.Signal, "the second signal did not end the process")
	assert.Equal(t, 143, exit.Code)
	assert.Equal(t, 1, strings.Count(stderr.String(), "ci: interrupted\n"), "one line says the run was interrupted")
	requireGone(t, grandchild)
	requireGone(t, pid)
}
