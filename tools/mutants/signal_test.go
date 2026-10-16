// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// helperEnv names the role of this test binary when a test runs it as a
// helper process; an empty value means it is the test run itself.
const helperEnv = "MUTANTS_TEST_HELPER"

// TestHelperProcess is not a test: the signal tests run this binary
// again with helperEnv set, to get a process with the signal handling
// of main that they can interrupt. It writes its own pid first, for the
// test to signal.
func TestHelperProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "main" {
		return
	}
	if err := os.WriteFile(os.Getenv("MUTANTS_TEST_MAINPID"), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		os.Exit(2)
	}
	e := env{
		dir:    os.Getenv("MUTANTS_TEST_DIR"),
		goBin:  os.Getenv("MUTANTS_TEST_GO"),
		tmpDir: os.Getenv("MUTANTS_TEST_TMP"),
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
	os.Exit(runMain(e, []string{os.Getenv("MUTANTS_TEST_MUTANTS")}))
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
	done           chan error
	stdout, stderr *bytes.Buffer
	pidfile        string // the pid of the hung test binary
	mainPidfile    string // the pid of the helper
}

// startHelper commits hangTest, hanging from the start (in the
// baseline) when inBaseline is set, and starts the command in a helper
// process.
func startHelper(t *testing.T, inBaseline bool) (fixture, helper) {
	t.Helper()
	f := newFixture(t, hangMutant)
	h := helper{done: make(chan error, 1), stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	h.pidfile = commitHang(t, f, inBaseline)
	h.mainPidfile = filepath.Join(t.TempDir(), "main")
	var gocache strings.Builder
	err := procgroup.Run(t.Context(), procgroup.Command{
		Class:  procgroup.Mise,
		Argv:   []string{f.e.goBin, "env", "GOCACHE"},
		Env:    os.Environ(),
		Stdout: &gocache,
	})
	require.NoError(t, err)
	// A process group of its own, killed when the test ends or at this
	// limit, so a helper the test lost does not outlive it.
	ctx := procgroup.WithTestTimeout(t.Context(), procgroup.ToolStep, 5*time.Minute)
	go func() {
		h.done <- procgroup.Run(ctx, procgroup.Command{
			Class: procgroup.ToolStep,
			Argv:  []string{os.Args[0], "-test.run=^TestHelperProcess$"},
			Env: append(append([]string{}, f.repo.Env...),
				"GOCACHE="+strings.TrimSpace(gocache.String()),
				helperEnv+"=main",
				"MUTANTS_TEST_MAINPID="+h.mainPidfile,
				"MUTANTS_TEST_DIR="+f.e.dir,
				"MUTANTS_TEST_GO="+f.e.goBin,
				"MUTANTS_TEST_TMP="+f.e.tmpDir,
				"MUTANTS_TEST_MUTANTS="+f.mutants,
			),
			Stdout: h.stdout,
			Stderr: h.stderr,
		})
	}()
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

// readPid waits for the file to hold a pid and returns it. A pid at or
// below 1 is never one a test may signal (kill(-1) reaches every
// process, kill(0) the test's own group), so it fails the test.
func readPid(t *testing.T, file string, wait time.Duration, what string) int {
	t.Helper()
	var pid int
	require.Eventually(t, func() bool {
		b, err := os.ReadFile(file)
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(string(b))
		return err == nil
	}, wait, 20*time.Millisecond, what)
	require.Greater(t, pid, 1, "the pid in %s", file)
	return pid
}

// hungPid waits for the hung test binary to write its pid. The handler
// of main is in place by then: it is set before the run starts.
func (h helper) hungPid(t *testing.T) int {
	t.Helper()
	pid := readPid(t, h.pidfile, 2*time.Minute, "the test binary hangs")
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	return pid
}

// signal sends sig to the helper, by the pid it wrote.
func (h helper) signal(t *testing.T, sig syscall.Signal) {
	t.Helper()
	require.NoError(t, syscall.Kill(readPid(t, h.mainPidfile, 10*time.Second, "the helper writes its pid"), sig))
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

// wait returns how the helper ended, or fails the test when it does not
// end in time.
func (h helper) wait(t *testing.T) *procgroup.ExitError {
	t.Helper()
	select {
	case err := <-h.done:
		exit, ok := errors.AsType[*procgroup.ExitError](err)
		require.True(t, ok, "the helper ends with a status: %v", err)
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
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			f, h := startHelper(t, false)
			before := f.state(t)
			pid := h.hungPid(t)
			h.signal(t, sig)
			assert.Equal(t, 128+int(sig), h.wait(t).Code)
			assert.Equal(t, "mutants: interrupted\n", h.ownStderr())
			assert.Empty(t, h.stdout.String())
			requireGone(t, pid)
			f.assertUntouched(t, before)
			f.assertNoStashNoClone(t)
		})
	}
}

// TestMutantsInterruptedInTheBaseline interrupts the baseline itself,
// before any mutant is written.
func TestMutantsInterruptedInTheBaseline(t *testing.T) {
	f, h := startHelper(t, true)
	before := f.state(t)
	pid := h.hungPid(t)
	h.signal(t, syscall.SIGINT)
	assert.Equal(t, 128+int(syscall.SIGINT), h.wait(t).Code)
	assert.Equal(t, "mutants: interrupted\n", h.ownStderr())
	requireGone(t, pid)
	f.assertUntouched(t, before)
}

// TestMutantsSecondSignal checks that a second signal does not leave
// the test binary behind: one that lands before the handler steps aside
// is absorbed and the status is the first's, and one after it ends the
// process by its default action. Either way the test binary is dead, as
// the handler kills its group before it steps aside.
func TestMutantsSecondSignal(t *testing.T) {
	_, h := startHelper(t, false)
	pid := h.hungPid(t)
	h.signal(t, syscall.SIGINT)
	h.signal(t, syscall.SIGINT)
	exit := h.wait(t)
	if exit.Signal != 0 {
		assert.Equal(t, syscall.SIGINT, exit.Signal)
	} else {
		assert.Equal(t, 128+int(syscall.SIGINT), exit.Code)
	}
	requireGone(t, pid)
}

// TestRunMainWithoutASignal checks that a run no signal touches keeps
// its own status and prints no interrupted line.
func TestRunMainWithoutASignal(t *testing.T) {
	f := newFixture(t, threeMutants)
	assert.Equal(t, exitFail, runMain(f.e, []string{f.mutants}))
	assert.NotContains(t, f.stderr.String(), "interrupted")
}
