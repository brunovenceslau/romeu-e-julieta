// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

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
const helperEnv = "MUTANTS_TEST_HELPER"

// TestHelperProcess is not a test: the signal tests run this binary
// again with helperEnv set, to get a process with the signal handling
// of main that they can interrupt.
func TestHelperProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "main" {
		return
	}
	e := env{
		dir:    os.Getenv("MUTANTS_TEST_DIR"),
		goBin:  os.Getenv("MUTANTS_TEST_GO"),
		tmpDir: os.Getenv("MUTANTS_TEST_TMP"),
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
	os.Exit(runSignalled(e, []string{os.Getenv("MUTANTS_TEST_MUTANTS")}))
}

// hangTest is a test file whose TestHang, when hang is true, writes the
// pid of the test binary to the file %q and then never ends.
const hangTest = `package calc

import (
	"os"
	"strconv"
	"testing"
)

const hang = %t

func TestHang(t *testing.T) {
	if hang {
		_ = os.WriteFile(%q, []byte(strconv.Itoa(os.Getpid())), 0o600)
		for {
		}
	}
}
`

const hangMutant = `mutants:
  - {id: hang, file: calc/hang_test.go, old: "const hang = false", new: "const hang = true", package: ./calc, run: ^TestHang$}
`

// helper is a run of the command in a helper process, with the signal
// handling of main.
type helper struct {
	cmd            *exec.Cmd
	done           chan error
	stdout, stderr *bytes.Buffer
	pidfile        string
}

// startHelper commits hangTest, hanging from the start (in the
// baseline) when inBaseline is set, and starts the command in a helper
// process.
func startHelper(t *testing.T, inBaseline bool) (fixture, helper) {
	t.Helper()
	f := newFixture(t, hangMutant)
	h := helper{done: make(chan error, 1), stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	h.pidfile = commitHang(t, f, inBaseline)
	gocache, err := exec.CommandContext(t.Context(), f.e.goBin, "env", "GOCACHE").Output()
	require.NoError(t, err)
	h.cmd = exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestHelperProcess$")
	h.cmd.Env = append(append([]string{}, f.repo.Env...),
		"GOCACHE="+strings.TrimSpace(string(gocache)),
		helperEnv+"=main",
		"MUTANTS_TEST_DIR="+f.e.dir,
		"MUTANTS_TEST_GO="+f.e.goBin,
		"MUTANTS_TEST_TMP="+f.e.tmpDir,
		"MUTANTS_TEST_MUTANTS="+f.mutants,
	)
	h.cmd.Stdout, h.cmd.Stderr = h.stdout, h.stderr
	require.NoError(t, h.cmd.Start())
	t.Cleanup(func() { _ = h.cmd.Process.Kill() })
	go func() { h.done <- h.cmd.Wait() }()
	return f, h
}

// commitHang commits hangTest to the fixture, hanging from the start
// when inBaseline is set, and returns the file its pid goes to.
func commitHang(t *testing.T, f fixture, inBaseline bool) string {
	t.Helper()
	pidfile := filepath.Join(t.TempDir(), "pid")
	src := strings.Replace(hangTest, "%t", strconv.FormatBool(inBaseline), 1)
	f.repo.Write("calc/hang_test.go", strings.Replace(src, "%q", strconv.Quote(pidfile), 1))
	f.repo.Commit("hang")
	return pidfile
}

// hungPid waits for the hung test binary to write its pid. The handler
// of main is in place by then: it is set before the run starts.
func (h helper) hungPid(t *testing.T) int {
	t.Helper()
	var pid int
	require.Eventually(t, func() bool {
		b, err := os.ReadFile(h.pidfile)
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(string(b))
		return err == nil
	}, 2*time.Minute, 20*time.Millisecond, "the test binary hangs")
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	return pid
}

// ownStderr is what the run wrote to standard error, without the line
// a test binary built with -cover adds when GOCOVERDIR is not set.
func (h helper) ownStderr() string {
	var b strings.Builder
	for line := range strings.Lines(h.stderr.String()) {
		if !strings.HasPrefix(line, "warning: GOCOVERDIR not set") {
			b.WriteString(line)
		}
	}
	return b.String()
}

// wait returns the exit status of the helper, or fails the test when it
// does not end in time.
func (h helper) wait(t *testing.T) *exec.ExitError {
	t.Helper()
	select {
	case err := <-h.done:
		var exit *exec.ExitError
		require.ErrorAs(t, err, &exit)
		return exit
	case <-time.After(time.Minute):
		require.FailNow(t, "the run did not stop")
		return nil
	}
}

// requireGone fails unless the process is dead: no such pid, or a
// zombie that /proc shows in state Z, as pid 1 of a container may not
// reap.
func requireGone(t *testing.T, pid int) {
	t.Helper()
	require.Eventually(t, func() bool {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			return true
		}
		b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
		if err != nil {
			return false
		}
		i := strings.LastIndex(string(b), ") ")
		return i >= 0 && i+2 < len(b) && b[i+2] == 'Z'
	}, 15*time.Second, 10*time.Millisecond, "the test binary %d is dead", pid)
}

// TestMutantsInterrupted sends each signal that ends a run while a
// mutant runs a test that never ends: the run kills the test binary,
// removes the clone, leaves the source as it was, prints only the
// interrupted line, and exits 128 plus the signal number.
func TestMutantsInterrupted(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP} {
		t.Run(sig.String(), func(t *testing.T) {
			f, h := startHelper(t, false)
			before := f.state(t)
			pid := h.hungPid(t)
			require.NoError(t, h.cmd.Process.Signal(sig))
			assert.Equal(t, exitSignalBase+int(sig), h.wait(t).ExitCode())
			assert.Equal(t, "mutants: interrupted\n", h.ownStderr())
			assert.Empty(t, h.stdout.String())
			requireGone(t, pid)
			f.assertUntouched(t, before)
		})
	}
}

// TestMutantsInterruptedInTheBaseline interrupts the baseline itself,
// before any mutant is written.
func TestMutantsInterruptedInTheBaseline(t *testing.T) {
	f, h := startHelper(t, true)
	before := f.state(t)
	pid := h.hungPid(t)
	require.NoError(t, h.cmd.Process.Signal(syscall.SIGINT))
	assert.Equal(t, exitSignalBase+int(syscall.SIGINT), h.wait(t).ExitCode())
	assert.Equal(t, "mutants: interrupted\n", h.ownStderr())
	requireGone(t, pid)
	f.assertUntouched(t, before)
}

// TestMutantsSecondSignal checks that a second signal ends the run at
// once: the handler steps aside after the first, so the second has its
// default action, unless the drain of the first ends the run before.
// Either way the test binary is dead, as the handler kills its group
// before it steps aside.
func TestMutantsSecondSignal(t *testing.T) {
	_, h := startHelper(t, false)
	pid := h.hungPid(t)
	require.NoError(t, h.cmd.Process.Signal(syscall.SIGINT))
	require.NoError(t, h.cmd.Process.Signal(syscall.SIGINT))
	exit := h.wait(t)
	status, ok := exit.Sys().(syscall.WaitStatus)
	require.True(t, ok)
	if status.Signaled() {
		assert.Equal(t, syscall.SIGINT, status.Signal())
	} else {
		assert.Equal(t, exitSignalBase+int(syscall.SIGINT), status.ExitStatus())
	}
	requireGone(t, pid)
}

// TestRunSignalledWithoutASignal checks that a run no signal touches
// keeps its own status.
func TestRunSignalledWithoutASignal(t *testing.T) {
	f := newFixture(t, threeMutants)
	assert.Equal(t, exitFail, runSignalled(f.e, []string{f.mutants}))
	assert.NotContains(t, f.stderr.String(), "interrupted")
}

// TestRunSignalledInProcess sends SIGTERM to this process while a
// mutant hangs, once the handler of runSignalled is in place (the hung
// test binary has written its pid): the run kills the binary and exits
// 128 plus the signal number.
func TestRunSignalledInProcess(t *testing.T) {
	f := newFixture(t, hangMutant)
	h := helper{pidfile: commitHang(t, f, false)}
	status := make(chan int, 1)
	go func() { status <- runSignalled(f.e, []string{f.mutants}) }()
	pid := h.hungPid(t)
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))
	select {
	case got := <-status:
		assert.Equal(t, exitSignalBase+int(syscall.SIGTERM), got)
	case <-time.After(time.Minute):
		require.FailNow(t, "the run did not stop")
	}
	assert.Equal(t, "mutants: interrupted\n", f.stderr.String())
	requireGone(t, pid)
	f.assertNoStashNoClone(t)
}

// TestGroupKill checks that kill ends the group of the command that
// runs, and does nothing when none runs.
func TestGroupKill(t *testing.T) {
	g := &group{}
	g.kill()
	done := make(chan error, 1)
	go func() {
		_, err := g.combinedOutput(exec.CommandContext(t.Context(), "sleep", "60"))
		done <- err
	}()
	require.Eventually(t, func() bool {
		g.mu.Lock()
		defer g.mu.Unlock()
		return g.pid != 0
	}, 10*time.Second, 10*time.Millisecond, "the command starts")
	g.kill()
	select {
	case err := <-done:
		var exit *exec.ExitError
		require.ErrorAs(t, err, &exit)
	case <-time.After(15 * time.Second):
		require.FailNow(t, "kill did not end the command")
	}
	assert.Zero(t, g.pid)
}
