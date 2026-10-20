// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package pin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteMakesItsTemporaryFileBesideTheTarget checks that a write to a
// nested path never needs the top of the root: it is made read-only, and
// the write still succeeds with mode 0644 over a file of mode 0600, with
// nothing left beside the target or in $TMPDIR.
func TestWriteMakesItsTemporaryFileBesideTheTarget(t *testing.T) {
	top := t.TempDir()
	nested := filepath.Join(top, "a", "b")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	target := filepath.Join(nested, "file")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o600))
	require.NoError(t, os.Chmod(target, 0o600))
	w, err := openWriter(top)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.root.Close() })
	require.NoError(t, os.Chmod(top, 0o555))
	t.Cleanup(func() { _ = os.Chmod(top, 0o755) })
	tmpdir := t.TempDir()
	t.Setenv("TMPDIR", tmpdir)

	require.NoError(t, w.write("a/b/file", []byte("new")))

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new", string(got))
	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
	for d, want := range map[string][]string{tmpdir: {}, nested: {"file"}, top: {"a"}} {
		entries, err := os.ReadDir(d)
		require.NoError(t, err)
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		assert.Equal(t, want, names, "the entries of %s", d)
	}
}
