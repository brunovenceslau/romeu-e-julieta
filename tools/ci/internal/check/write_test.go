// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteIfChangedReportsAnUnwritableTarget checks the error paths of
// the writer that the fixtures above cannot reach.
func TestWriteIfChangedReportsAnUnwritableTarget(t *testing.T) {
	dir := t.TempDir()
	root := citest.RootOf(t, dir)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "d"), 0o755))
	_, err := check.WriteIfChanged(root, "d", []byte("x"))
	require.Error(t, err, "a directory is not a file to write")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "the temporary file of the failed write is gone")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f"), nil, 0o644))
	_, err = check.WriteIfChanged(root, "f/child", []byte("x"))
	require.Error(t, err, "a file is not a directory to create in")
}

// TestWriteIfChangedStaysInsideTheRoot checks that a symbolic link at the
// target, or in its directory, never sends a write outside the root: the
// link to a file outside is replaced, and the link to a directory
// outside is an error that writes nothing there.
func TestWriteIfChangedStaysInsideTheRoot(t *testing.T) {
	t.Run("a link to a file outside is replaced", func(t *testing.T) {
		dir, outside := t.TempDir(), filepath.Join(t.TempDir(), "victim")
		require.NoError(t, os.WriteFile(outside, []byte("mine\n"), 0o644))
		require.NoError(t, os.Symlink(outside, filepath.Join(dir, "out.md")))
		wrote, err := check.WriteIfChanged(citest.RootOf(t, dir), "out.md", []byte("generated\n"))
		require.NoError(t, err)
		assert.True(t, wrote)
		info, err := os.Lstat(filepath.Join(dir, "out.md"))
		require.NoError(t, err)
		assert.True(t, info.Mode().IsRegular(), "a regular file now")
		b, err := os.ReadFile(outside)
		require.NoError(t, err)
		assert.Equal(t, "mine\n", string(b), "the file outside is untouched")
	})
	t.Run("a hard link to a file outside is replaced, not written through", func(t *testing.T) {
		dir, outsideDir := t.TempDir(), t.TempDir()
		outside := filepath.Join(outsideDir, "victim")
		require.NoError(t, os.WriteFile(outside, []byte("mine\n"), 0o644))
		require.NoError(t, os.Link(outside, filepath.Join(dir, "out.md")))
		wrote, err := check.WriteIfChanged(citest.RootOf(t, dir), "out.md", []byte("generated\n"))
		require.NoError(t, err)
		assert.True(t, wrote)
		b, err := os.ReadFile(outside)
		require.NoError(t, err)
		assert.Equal(t, "mine\n", string(b), "the other name of the file keeps its bytes")
		b, err = os.ReadFile(filepath.Join(dir, "out.md"))
		require.NoError(t, err)
		assert.Equal(t, "generated\n", string(b))
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Len(t, entries, 1, "no temporary file is left")
	})
	t.Run("a link to a directory outside is an error", func(t *testing.T) {
		dir, outside := t.TempDir(), t.TempDir()
		require.NoError(t, os.Symlink(outside, filepath.Join(dir, "sub")))
		_, err := check.WriteIfChanged(citest.RootOf(t, dir), "sub/out.md", []byte("generated\n"))
		require.Error(t, err)
		assert.NoFileExists(t, filepath.Join(outside, "out.md"))
	})
	t.Run("a link to an identical copy inside is replaced", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "copy"), []byte("generated\n"), 0o644))
		require.NoError(t, os.Symlink("copy", filepath.Join(dir, "out.md")))
		wrote, err := check.WriteIfChanged(citest.RootOf(t, dir), "out.md", []byte("generated\n"))
		require.NoError(t, err)
		assert.True(t, wrote, "the link is not current, whatever it reads")
		info, err := os.Lstat(filepath.Join(dir, "out.md"))
		require.NoError(t, err)
		assert.True(t, info.Mode().IsRegular())
	})
}

// TestWriteIfChangedDropsTheExecuteBit checks that a generated file with
// the right bytes and an execute bit is put back to 0644, since git
// tracks the bit.
func TestWriteIfChangedDropsTheExecuteBit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o755))
	require.NoError(t, os.Chmod(path, 0o755))
	wrote, err := check.WriteIfChanged(citest.RootOf(t, dir), "f", []byte("x"))
	require.NoError(t, err)
	assert.True(t, wrote)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}
