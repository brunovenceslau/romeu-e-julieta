// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDoctorSettingsMode: host settings that are not 0600, or a state
// directory that is not 0700, fail settings-mode; doctor changes
// neither mode (REQ-0381).
func TestDoctorSettingsMode(t *testing.T) {
	t.Parallel()
	t.Run("the state directory", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.acknowledge(t, f.sbx.release)
		f.acknowledged = false
		require.NoError(t, os.Chmod(f.stateDir(), 0o755))

		rep, out, _ := f.output(t)

		assert.Equal(t, resultFail, check(t, rep, "settings-mode").Result, out)
		info, err := os.Stat(f.stateDir())
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o755), info.Mode().Perm(), "doctor writes nothing")
	})
	t.Run("the settings file", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.settingsMode = 0o644

		rep, out, _ := f.output(t)

		assert.Equal(t, resultFail, check(t, rep, "settings-mode").Result, out)
	})
}
