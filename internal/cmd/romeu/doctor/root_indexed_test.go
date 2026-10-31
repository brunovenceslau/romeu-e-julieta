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
)

// TestDoctorRootIndexed: a <name>-env/ without its .metadata_never_index
// marker, or a root under a folder a cloud service syncs, is the warning
// root-indexed, which changes no exit (REQ-0376).
func TestDoctorRootIndexed(t *testing.T) {
	t.Parallel()
	t.Run("doctor warns root-indexed when the marker is missing", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		require.NoError(t, os.Mkdir(filepath.Join(f.root, "shop-env"), 0o700))

		rep, out, err := f.output(t)

		require.NoError(t, err, out)
		c := check(t, rep, "root-indexed")
		assert.Equal(t, resultWarn, c.Result)
		assert.Contains(t, c.Message, "shop-env")
	})
	t.Run("a marker keeps it quiet", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		require.NoError(t, os.Mkdir(filepath.Join(f.root, "shop-env"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(f.root, "shop-env", neverIndex), nil, 0o600))

		rep, out, _ := f.output(t)

		assert.Equal(t, resultPass, check(t, rep, "root-indexed").Result, out)
	})
	t.Run("a root under Dropbox", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.settings.Root = filepath.Join(f.home, "Dropbox", "dev")
		require.NoError(t, os.MkdirAll(f.settings.Root, 0o700))

		rep, out, _ := f.output(t)

		c := check(t, rep, "root-indexed")
		assert.Equal(t, resultWarn, c.Result, out)
		assert.Contains(t, c.Message, "Dropbox")
	})
}
