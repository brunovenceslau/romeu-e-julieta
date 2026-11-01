// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package initcmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
)

// TestInitWritesTheMarker: after init, <cfg>-env/.romeu-config holds the
// dir of the clone on one line, mode 0600, outside the clone (02 2.3).
func TestInitWritesTheMarker(t *testing.T) {
	m := newMachine(t, projectSpec)

	code, _, stderr := m.init(t, "--config-url", "https://github.com/owner/cfg")

	require.Equal(t, 0, code, stderr)
	marker := filepath.Join(m.home, "Developer", "cfg-env", MarkerName)
	data, err := os.ReadFile(marker)
	require.NoError(t, err)
	assert.Equal(t, "cfg\n", string(data))
	fi, err := os.Lstat(marker)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
	assert.NoFileExists(t, filepath.Join(m.home, "Developer", "cfg-env", "cfg", MarkerName), "the marker is outside the clone")
}

// TestInitRefusesAMarkerThatExists: a marker left under <cfg>-env/ before
// init is refused by the slug config-marker, exit 2, with the clone of
// this run removed and the marker as it was.
func TestInitRefusesAMarkerThatExists(t *testing.T) {
	m := newMachine(t, projectSpec)
	env := filepath.Join(m.home, "Developer", "cfg-env")
	require.NoError(t, os.MkdirAll(env, 0o700))
	marker := filepath.Join(env, MarkerName)
	require.NoError(t, os.WriteFile(marker, []byte("theirs\n"), 0o600))

	code, _, stderr := m.init(t, "--config-url", "https://github.com/owner/cfg")

	assert.Equal(t, 2, code, stderr)
	assert.Contains(t, stderr, "config-marker")
	data, err := os.ReadFile(marker)
	require.NoError(t, err)
	assert.Equal(t, "theirs\n", string(data))
	assert.NoDirExists(t, filepath.Join(env, "cfg"))
	assert.NoFileExists(t, m.settings())
}

// TestInitFailedAfterTheCloneLeavesNoMarker: an init that fails at the
// settings, after the clone, leaves no marker, as it leaves no clone.
func TestInitFailedAfterTheCloneLeavesNoMarker(t *testing.T) {
	m := newMachine(t, projectSpec)
	clone := cloneRepo
	cloneRepo = func(ctx context.Context, g *gitsafe.Runner, url, dest, helper string) error {
		if err := clone(ctx, g, url, dest, helper); err != nil {
			return err
		}
		return os.WriteFile(m.settings(), []byte("theirs"), 0o600)
	}

	code, _, stderr := m.init(t, "--config-url", "https://github.com/owner/cfg")

	assert.Equal(t, 2, code, stderr)
	assert.NoFileExists(t, filepath.Join(m.home, "Developer", "cfg-env", MarkerName))
	assert.NoDirExists(t, filepath.Join(m.home, "Developer", "cfg-env", "cfg"))
}
