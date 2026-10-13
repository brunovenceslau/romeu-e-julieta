// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTimeouts pins the timeout table, the starting values of 04 4.1,
// Starting values. The salvage exec, the hook stages and the pane steps
// have no timeout.
func TestTimeouts(t *testing.T) {
	want := map[Class]time.Duration{
		SbxRead:      30 * time.Second,
		SbxChange:    2 * time.Minute,
		SbxStart:     5 * time.Minute,
		SbxCreate:    30 * time.Minute,
		JulietaSetup: 30 * time.Minute,
		JulietaExec:  5 * time.Minute,
		HostGitFetch: 15 * time.Minute,
		Secret:       30 * time.Second,
		JulietaGit:   5 * time.Minute,
		Mise:         30 * time.Minute,
		Herdr:        30 * time.Second,
		Unbounded:    0,
	}
	assert.Len(t, timeouts, len(want), "every class has a row, and every row a class")
	for class, limit := range want {
		assert.Equal(t, limit, class.Timeout(), "class %s", class)
	}
	assert.Zero(t, Unbounded.Timeout(), "the salvage class has no timeout")
	assert.Zero(t, classNone.Timeout())
}

// TestClassString checks that each class has a name of its own, which
// the details of subprocess-timeout print.
func TestClassString(t *testing.T) {
	seen := map[string]Class{}
	for class := range timeouts {
		name := class.String()
		assert.NotEmpty(t, name)
		assert.NotContains(t, seen, name, "the name of class %d is unique", int(class))
		seen[name] = class
	}
	assert.Equal(t, "sbx-read", SbxRead.String())
	assert.Equal(t, "class-99", Class(99).String())
}

// TestWithTestTimeout checks that the seam shortens one class for the
// context it returns and leaves the others, and the table, as they are.
func TestWithTestTimeout(t *testing.T) {
	ctx := WithTestTimeout(t.Context(), SbxRead, time.Millisecond)
	assert.Equal(t, time.Millisecond, limitOf(ctx, SbxRead))
	assert.Equal(t, 2*time.Minute, limitOf(ctx, SbxChange))
	assert.Equal(t, 30*time.Second, limitOf(t.Context(), SbxRead))
	assert.Equal(t, 30*time.Second, SbxRead.Timeout())
	nested := WithTestTimeout(ctx, SbxChange, time.Second)
	assert.Equal(t, time.Millisecond, limitOf(nested, SbxRead), "an outer setting stays")
	assert.Equal(t, time.Second, limitOf(nested, SbxChange))
}

// TestWithTestTimeoutAtOrBelowZero checks that a limit at or below zero
// set through the seam means no timeout, as a zero does in the table.
func TestWithTestTimeoutAtOrBelowZero(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Second} {
		ctx := WithTestTimeout(t.Context(), SbxRead, d)
		assert.Equal(t, d, limitOf(ctx, SbxRead))
		err := Run(ctx, Command{Class: SbxRead, Argv: []string{"/bin/sh", "-c", "sleep 0.1"}, Env: shellEnv})
		require.NoError(t, err, "limit %s", d)
	}
}
