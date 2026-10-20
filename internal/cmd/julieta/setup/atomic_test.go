// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
)

// TestWriteStateKeepsItsTemporaryFileAndModeToItself checks that the
// temporary file of writeState is made in the state directory and never
// in $TMPDIR, and that the file ends with mode 0600 over an existing
// file of mode 0644.
func TestWriteStateKeepsItsTemporaryFileAndModeToItself(t *testing.T) {
	home := t.TempDir()
	dir := setup.StateDir(home)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	target := filepath.Join(dir, "state.json")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o644))
	require.NoError(t, os.Chmod(target, 0o644))
	tmpdir := t.TempDir()
	t.Setenv("TMPDIR", tmpdir)

	require.NoError(t, setup.WriteState(home, "state.json", []byte("new")))

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new", string(got))
	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	assertOnly(t, tmpdir)
	assertOnly(t, dir, "state.json")
}

// assertOnly checks that dir holds the entries named, and nothing else.
func assertOnly(t *testing.T, dir string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	got := []string{}
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if names == nil {
		names = []string{}
	}
	assert.Equal(t, names, got, "the entries of %s", dir)
}
