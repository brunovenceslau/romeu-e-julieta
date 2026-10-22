// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// The environment of a helper process, which is this test binary run
// again, as the tests of procgroup do: the state directory it locks, and
// the file it writes the error id to, since a command's output goes
// through the CLI writer and a test helper has none.
const (
	lockHelperEnv = "STATE_TEST_LOCK_DIR"
	lockOutEnv    = "STATE_TEST_LOCK_OUT"
	// lockHoldEnv names the file a helper that holds the lock creates
	// once it holds it; the helper then waits to be killed.
	lockHoldEnv = "STATE_TEST_LOCK_HOLD"
)

// TestLockHelperProcess is not a test: lockAnother runs this binary again
// with lockHelperEnv set, to get a second romeu process that takes the
// lock, and exits with the status the error of the lock maps to.
func TestLockHelperProcess(t *testing.T) {
	dir := os.Getenv(lockHelperEnv)
	if dir == "" {
		return
	}
	s, err := Open(dir)
	if err != nil {
		os.Exit(2)
	}
	lk, err := s.Lock()
	if err == nil && os.Getenv(lockHoldEnv) != "" {
		if os.WriteFile(os.Getenv(lockHoldEnv), nil, 0o600) != nil {
			os.Exit(2)
		}
		time.Sleep(time.Minute)
		os.Exit(3)
	}
	if err != nil {
		if os.WriteFile(os.Getenv(lockOutEnv), []byte(cli.Of(err).ID), 0o600) != nil {
			os.Exit(2)
		}
		os.Exit(cli.ExitCode(err))
	}
	lk.Unlock()
	os.Exit(0)
}

// lockAnother runs a second process that takes romeu.lock of dir, and
// returns its exit status and the error id it wrote, empty when none.
func lockAnother(t *testing.T, dir string) (status int, id string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "id")
	err := procgroup.Run(t.Context(), procgroup.Command{
		Class: procgroup.ToolProbe,
		Argv:  []string{os.Args[0], "-test.run=^TestLockHelperProcess$"},
		Env:   append(os.Environ(), "GORACE=atexit_sleep_ms=0", lockHelperEnv+"="+dir, lockOutEnv+"="+out),
	})
	written, _ := os.ReadFile(out)
	if exit, ok := errors.AsType[*procgroup.ExitError](err); ok {
		return exit.Code, string(written)
	}
	require.NoError(t, err)
	return 0, string(written)
}

func TestStore_Lock(t *testing.T) {
	t.Run("a second process exits 1 with RJ-101", func(t *testing.T) {
		s := openTemp(t)
		lk, err := s.Lock()
		require.NoError(t, err)
		status, id := lockAnother(t, s.Dir())
		assert.Equal(t, 1, status)
		assert.Equal(t, "RJ-101", id)

		lk.Unlock()
		status, _ = lockAnother(t, s.Dir())
		assert.Equal(t, 0, status, "the lock is free after unlock")
	})
	t.Run("the error is the row locked, exit 1", func(t *testing.T) {
		s := openTemp(t)
		lk, err := s.Lock()
		require.NoError(t, err)
		defer lk.Unlock()
		_, err = s.Lock()
		require.Error(t, err)
		assert.Equal(t, "RJ-101", cli.Of(err).ID)
		assert.Equal(t, 1, cli.ExitCode(err))
	})
	t.Run("the lock file is 0600", func(t *testing.T) {
		s := openTemp(t)
		lk, err := s.Lock()
		require.NoError(t, err)
		defer lk.Unlock()
		info, err := os.Stat(s.Dir() + "/romeu.lock")
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})
	t.Run("a closed store cannot lock", func(t *testing.T) {
		s := openTemp(t)
		require.NoError(t, s.Close())
		_, err := s.Lock()
		require.Error(t, err)
	})
	t.Run("a holder that is killed leaves no lock behind", func(t *testing.T) {
		s := openTemp(t)
		ready := filepath.Join(t.TempDir(), "ready")
		ctx, kill := context.WithCancel(t.Context())
		defer kill()
		ended := make(chan struct{})
		go func() {
			defer close(ended)
			_ = procgroup.Run(ctx, procgroup.Command{
				Class: procgroup.ToolProbe,
				Argv:  []string{os.Args[0], "-test.run=^TestLockHelperProcess$"},
				Env:   append(os.Environ(), "GORACE=atexit_sleep_ms=0", lockHelperEnv+"="+s.Dir(), lockHoldEnv+"="+ready),
			})
		}()
		require.Eventually(t, func() bool { _, err := os.Stat(ready); return err == nil }, 20*time.Second, 10*time.Millisecond)
		_, err := s.Lock()
		require.Error(t, err, "the holder has it")
		assert.Equal(t, "RJ-101", cli.Of(err).ID)

		kill()
		<-ended
		lk, err := s.Lock()
		require.NoError(t, err, "the kernel released the lock with the process (REQ-0283)")
		lk.Unlock()
	})
}
