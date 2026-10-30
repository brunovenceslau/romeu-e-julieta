// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tree returns the path and the mode of every entry below dir, dir
// included, so a test can show that a read changed nothing.
func tree(t *testing.T, dir string) map[string]fs.FileMode {
	t.Helper()
	got := map[string]fs.FileMode{}
	require.NoError(t, filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		got[p] = info.Mode()
		return nil
	}))
	return got
}

func TestOpenRead(t *testing.T) {
	t.Parallel()
	t.Run("an absent directory is not made", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join(t.TempDir(), "romeu")
		_, err := OpenRead(dir)
		require.ErrorIs(t, err, fs.ErrNotExist)
		_, err = os.Stat(dir)
		require.ErrorIs(t, err, fs.ErrNotExist)
	})
	t.Run("a directory without subdirectories holds no record and gains none", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.Chmod(dir, 0o755))
		before := tree(t, dir)
		s, err := OpenRead(dir)
		require.NoError(t, err)
		defer func() { require.NoError(t, s.Close()) }()
		names, err := s.ProjectNames()
		require.NoError(t, err)
		assert.Empty(t, names)
		_, err = s.LoadProject("shop")
		require.ErrorIs(t, err, fs.ErrNotExist)
		_, err = s.LoadToolchain()
		require.ErrorIs(t, err, fs.ErrNotExist)
		assert.Equal(t, before, tree(t, dir), "the mode 0755 stays for doctor to report")
	})
	t.Run("a store opened to read cannot be locked", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		s, err := OpenRead(dir)
		require.NoError(t, err)
		defer func() { require.NoError(t, s.Close()) }()
		_, err = s.Lock()
		require.ErrorIs(t, err, ErrReadOnly)
		_, err = os.Stat(filepath.Join(dir, lockName))
		require.ErrorIs(t, err, fs.ErrNotExist)
	})
	t.Run("a record reads as Open reads it", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		seed(t, dir, "shop")
		s, err := OpenRead(dir)
		require.NoError(t, err)
		defer func() { require.NoError(t, s.Close()) }()
		p, err := s.LoadProject("shop")
		require.NoError(t, err)
		assert.Equal(t, ProjectField, p.Schema)
	})
}

// seed writes the fixture record of the project shop under dir as
// projects/<name>.json, made by Open as romeu makes it.
func seed(t *testing.T, dir, name string) {
	t.Helper()
	s, err := Open(dir)
	require.NoError(t, err)
	require.NoError(t, s.Close())
	data, err := os.ReadFile(filepath.Join("testdata", "project.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, projectsDir, name+".json"), data, FileMode))
}

func TestStoreProjectNames(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seed(t, dir, "shop")
	seed(t, dir, "blog")
	projects := filepath.Join(dir, projectsDir)
	for _, name := range []string{".shop.json123", "Shop.json", "notes.txt", "-x.json"} {
		require.NoError(t, os.WriteFile(filepath.Join(projects, name), []byte("{}"), FileMode))
	}
	require.NoError(t, os.Mkdir(filepath.Join(projects, "dir.json"), DirMode))
	s, err := OpenRead(dir)
	require.NoError(t, err)
	defer func() { require.NoError(t, s.Close()) }()
	names, err := s.ProjectNames()
	require.NoError(t, err)
	assert.Equal(t, []string{"blog", "shop"}, names, "dotfiles, other names and directories are left out")
}
