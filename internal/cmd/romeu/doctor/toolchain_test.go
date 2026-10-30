// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
)

// TestDoctorToolchain: the acknowledgement of gate 1 missing or stale
// fails toolchain-approval-required, by gate 1's own check (REQ-0369).
func TestDoctorToolchain(t *testing.T) {
	t.Parallel()
	t.Run("no state directory", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.acknowledged = false

		rep, out, err := f.output(t)

		assert.Equal(t, resultFail, check(t, rep, "toolchain-approval-required").Result, out)
		assert.Equal(t, 6, exitOf(err))
		assert.NoDirExists(t, f.stateDir(), "doctor makes no state directory")
	})
	t.Run("stale", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.acknowledged = false
		f.acknowledge(t, sbxdrv.Release{Major: 0, Minor: 49, Patch: 1})

		rep, out, _ := f.output(t)

		c := check(t, rep, "toolchain-approval-required")
		assert.Equal(t, resultFail, c.Result, out)
		assert.Contains(t, c.Message, "sbxVersion")
	})
	t.Run("a record that does not decode leaves it unrun", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.acknowledged = false
		f.store(t)
		require.NoError(t, os.WriteFile(filepath.Join(f.stateDir(), "toolchain.json"), []byte("{"), 0o600))

		rep, out, _ := f.output(t)

		assert.Equal(t, "record-unreadable", check(t, rep, "toolchain-approval-required").Error, out)
	})
}
