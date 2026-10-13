// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTimeoutError shortens one class through the seam and checks that
// a command past its limit fails with subprocess-timeout (RJ-323),
// whose details name the class and its limit, and that its group is
// killed.
func TestTimeoutError(t *testing.T) {
	pidfile := filepath.Join(t.TempDir(), "pid")
	killAtEnd(t, pidfile)
	ctx := WithTestTimeout(t.Context(), SbxRead, 300*time.Millisecond)
	start := time.Now()
	err := Run(ctx, Command{Class: SbxRead, Argv: []string{"/bin/sh", "-c", grandchildScript, pidfile}, Env: shellEnv})
	var timeout *TimeoutError
	require.ErrorAs(t, err, &timeout)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 5*time.Second)
	assert.Equal(t, "RJ-323", timeout.ID())
	assert.Equal(t, "subprocess-timeout", timeout.Slug())
	assert.Equal(t, 1, timeout.ExitCode())
	assert.Equal(t, SbxRead, timeout.Class)
	assert.Equal(t, 300*time.Millisecond, timeout.Limit)
	assert.Equal(t, "class sbx-read, limit 300ms", timeout.Details())
	assert.Equal(t, "/bin/sh ran past the 300ms limit of class sbx-read", timeout.Error())
	assert.Contains(t, timeout.Fix(), "sbx ls --json")
	assertGone(t, waitForPid(t, pidfile))
}

// TestTimeoutErrorOnlyForTheClassLimit checks that a deadline of the
// caller, which fires before the limit of the class, is the caller's
// and not subprocess-timeout.
func TestTimeoutErrorOnlyForTheClassLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	err := Run(ctx, Command{Class: SbxRead, Argv: []string{"/bin/sh", "-c", "sleep 60"}, Env: shellEnv})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	var timeout *TimeoutError
	assert.NotErrorAs(t, err, &timeout)
}
