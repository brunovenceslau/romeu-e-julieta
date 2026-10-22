// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openTemp opens a store in a new directory.
func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "romeu"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	return s
}

func TestWriteFile(t *testing.T) {
	t.Run("writes a file of mode 0600 whatever the umask, and replaces it whole", func(t *testing.T) {
		s := openTemp(t)
		require.NoError(t, writeFile(s.root, "a.json", []byte("one")))
		require.NoError(t, writeFile(s.root, "a.json", []byte("two")))
		info, err := os.Stat(filepath.Join(s.Dir(), "a.json"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		got, err := os.ReadFile(filepath.Join(s.Dir(), "a.json"))
		require.NoError(t, err)
		assert.Equal(t, "two", string(got))
	})
	t.Run("leaves no temporary file beside it", func(t *testing.T) {
		s := openTemp(t)
		require.NoError(t, writeFile(s.projects, "shop.json", []byte("x")))
		entries, err := os.ReadDir(filepath.Join(s.Dir(), "projects"))
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, "shop.json", entries[0].Name())
	})
	t.Run("a failed write leaves the old file and no temporary file", func(t *testing.T) {
		s := openTemp(t)
		require.NoError(t, writeFile(s.projects, "shop.json", []byte("old")))
		// A directory where the file goes makes the rename fail.
		require.NoError(t, os.Mkdir(filepath.Join(s.Dir(), "projects", "bad.json"), 0o700))
		require.Error(t, writeFile(s.projects, "bad.json", []byte("x")))
		got, err := os.ReadFile(filepath.Join(s.Dir(), "projects", "shop.json"))
		require.NoError(t, err)
		assert.Equal(t, "old", string(got))
		entries, err := os.ReadDir(filepath.Join(s.Dir(), "projects"))
		require.NoError(t, err)
		assert.Len(t, entries, 2, "shop.json and the directory bad.json, no temporary file")
	})
	t.Run("a name cannot leave the directory", func(t *testing.T) {
		s := openTemp(t)
		require.Error(t, writeFile(s.projects, "../escape.json", []byte("x")))
		assert.NoFileExists(t, filepath.Join(s.Dir(), "escape.json"))
	})
	t.Run("a closed directory cannot be written", func(t *testing.T) {
		s := openTemp(t)
		require.NoError(t, s.Close())
		require.Error(t, writeFile(s.projects, "a.json", []byte("x")))
		require.Error(t, syncDir(s.projects))
	})
}
