// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build linux || darwin

package hygiene

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteFileKeepsItsTemporaryFileAndModeToItself checks that the
// temporary file of writeFile is made beside the target and never in
// $TMPDIR, and that the file ends with mode 0644 over one of mode 0600.
func TestWriteFileKeepsItsTemporaryFileAndModeToItself(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "denylist")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o600))
	require.NoError(t, os.Chmod(target, 0o600))
	tmpdir := t.TempDir()
	t.Setenv("TMPDIR", tmpdir)

	require.NoError(t, writeFile(target, []byte("new")))

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new", string(got))
	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
	for dir, want := range map[string][]string{tmpdir: {}, filepath.Dir(target): {"denylist"}} {
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		got := []string{}
		for _, e := range entries {
			got = append(got, e.Name())
		}
		assert.Equal(t, want, got, "the entries of %s", dir)
	}
}
