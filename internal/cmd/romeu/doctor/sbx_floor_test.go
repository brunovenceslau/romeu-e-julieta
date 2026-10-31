// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
)

// TestDoctorSbxFloor: sbx below the floor of 12 12.1, or not at the
// path of the settings, fails sbx-floor (REQ-0366).
func TestDoctorSbxFloor(t *testing.T) {
	t.Parallel()
	t.Run("below the floor", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.sbx.release = sbxdrv.Release{Major: 0, Minor: 40, Patch: 0}

		rep, out, _ := f.output(t)

		c := check(t, rep, "sbx-floor")
		assert.Equal(t, resultFail, c.Result, out)
		assert.Contains(t, c.Message, "0.40.0")
	})
	t.Run("not at the path of the settings", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		require.NoError(t, os.Remove(f.settings.Tools.Sbx))

		rep, out, _ := f.output(t)

		assert.Equal(t, resultFail, check(t, rep, "sbx-floor").Result, out)
	})
}
