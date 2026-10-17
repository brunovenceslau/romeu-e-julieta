// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMainSignalRule is the signal rule of 04 4.1, Signals, run on a
// helper process that installs the handler through Main.
func TestMainSignalRule(t *testing.T) {
	t.Run("a signal ends the group and exits 143", func(t *testing.T) {
		cmd, pidfile, _ := helper(t, "group")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		require.NoError(t, cmd.Start())
		pid := waitForPid(t, pidfile)
		require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
		assert.Equal(t, 143, exitCode(t, cmd.Wait()))
		assert.Equal(t, helperName+": interrupted\n", stderr.String())
		assertGone(t, pid)
	})
	t.Run("SIGINT exits 130", func(t *testing.T) {
		cmd, pidfile, _ := helper(t, "group")
		require.NoError(t, cmd.Start())
		pid := waitForPid(t, pidfile)
		require.NoError(t, cmd.Process.Signal(syscall.SIGINT))
		assert.Equal(t, 130, exitCode(t, cmd.Wait()))
		assertGone(t, pid)
	})
	t.Run("a command that had already failed with 2 exits 143", func(t *testing.T) {
		cmd, _, ready := helper(t, "failed")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		require.NoError(t, cmd.Start())
		waitForFile(t, ready)
		require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
		assert.Equal(t, 143, exitCode(t, cmd.Wait()))
		assert.Equal(t, helperName+": interrupted\n", stderr.String())
	})
	t.Run("a command that finished keeps 0", func(t *testing.T) {
		cmd, _, ready := helper(t, "finished")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		require.NoError(t, cmd.Start())
		waitForFile(t, ready)
		require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
		assert.Equal(t, 0, exitCode(t, cmd.Wait()))
		assert.Empty(t, stderr.String())
	})
	t.Run("a second signal before the handler steps aside is absorbed", func(t *testing.T) {
		// The helper holds its handler before it steps aside, sends
		// itself SIGINT there and waits until the handler's channel has
		// it; the status stays 128 plus SIGTERM.
		cmd, _, ready := helper(t, "absorb")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		require.NoError(t, cmd.Start())
		waitForFile(t, ready)
		require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
		assert.Equal(t, 143, exitCode(t, cmd.Wait()))
		assert.Equal(t, helperName+": interrupted\n", stderr.String())
		assert.FileExists(t, filepath.Join(filepath.Dir(ready), "marker"), "the handler went on after the second signal")
	})
	t.Run("the status waits for the handler to kill the groups", func(t *testing.T) {
		// The command returns at the cancel, while the handler is still
		// held; Main returns only once the handler is done.
		cmd, _, ready := helper(t, "stepped")
		require.NoError(t, cmd.Start())
		waitForFile(t, ready)
		require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
		assert.Equal(t, 143, exitCode(t, cmd.Wait()))
		assert.FileExists(t, filepath.Join(filepath.Dir(ready), "marker"))
	})
	t.Run("a signal after the handler steps aside takes its default action", func(t *testing.T) {
		cmd, _, ready := helper(t, "deaf")
		require.NoError(t, cmd.Start())
		waitForFile(t, ready)
		require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
		errc := make(chan error, 1)
		go func() { errc <- cmd.Wait() }()
		// The handler steps aside right after the first signal; until it
		// has, a further signal is absorbed, so the test repeats it.
		tick := time.NewTicker(20 * time.Millisecond)
		defer tick.Stop()
		deadline := time.After(10 * time.Second)
		for {
			select {
			case err := <-errc:
				var exit *exec.ExitError
				require.ErrorAs(t, err, &exit)
				status, ok := exit.Sys().(syscall.WaitStatus)
				require.True(t, ok)
				assert.True(t, status.Signaled(), "the process was ended by a signal, not by an exit")
				assert.Equal(t, syscall.SIGTERM, status.Signal())
				return
			case <-tick.C:
				_ = cmd.Process.Signal(syscall.SIGTERM)
			case <-deadline:
				require.FailNow(t, "the second signal did not end the process")
			}
		}
	})
}

// TestMainWithoutSignal checks that a command no signal touched keeps
// its status and says nothing, and that the commands it runs are
// tracked for the handler and forgotten once they end.
func TestMainWithoutSignal(t *testing.T) {
	var stderr bytes.Buffer
	status := Main(helperName, &stderr, func(ctx context.Context) int {
		g, ok := ctx.Value(groupsKey{}).(*groups)
		require.True(t, ok, "Main hands its groups to Run through the context")
		require.NoError(t, Run(ctx, Command{Class: JulietaGit, Argv: []string{"/bin/sh", "-c", "exit 0"}, Env: shellEnv}))
		g.mu.Lock()
		defer g.mu.Unlock()
		assert.Empty(t, g.leaders, "a group that ended is forgotten")
		return 2
	})
	assert.Equal(t, 2, status)
	assert.Empty(t, stderr.String())
}

// exitCode returns the exit status of a helper that exited.
func exitCode(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	require.ErrorAs(t, err, &exit, "the helper ran")
	return exit.ExitCode()
}
