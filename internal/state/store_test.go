// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultDir(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
		err  error
	}{
		{"xdg state home", map[string]string{"XDG_STATE_HOME": "/x/state", "HOME": "/h"}, "/x/state/romeu", nil},
		{"home fallback", map[string]string{"HOME": "/h"}, "/h/.local/state/romeu", nil},
		{"a relative xdg path counts as unset", map[string]string{"XDG_STATE_HOME": "rel", "HOME": "/h"}, "/h/.local/state/romeu", nil},
		{"nothing absolute", map[string]string{"HOME": "rel"}, "", ErrNoStateDir},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DefaultDir(func(k string) string { return tt.env[k] })
			require.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOpen(t *testing.T) {
	t.Run("makes the directories 0700", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "state", "romeu")
		s, err := Open(dir)
		require.NoError(t, err)
		defer func() { require.NoError(t, s.Close()) }()
		for _, p := range []string{dir, filepath.Join(dir, "projects"), filepath.Join(dir, "descriptors")} {
			info, err := os.Stat(p)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o700), info.Mode().Perm(), p)
		}
	})
	t.Run("narrows an existing directory to 0700", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Chmod(dir, 0o755))
		s, err := Open(dir)
		require.NoError(t, err)
		defer func() { require.NoError(t, s.Close()) }()
		info, err := os.Stat(dir)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	})
	t.Run("a path under a file is an error", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(file, nil, 0o600))
		_, err := Open(filepath.Join(file, "romeu"))
		require.Error(t, err)
	})
	t.Run("a file where a subdirectory goes is an error", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "projects"), nil, 0o600))
		_, err := Open(dir)
		require.Error(t, err)
	})
	t.Run("a path that is a file is an error", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(file, nil, 0o600))
		_, err := Open(file)
		require.Error(t, err)
	})
	t.Run("narrows an existing projects directory", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, "projects"), 0o755))
		s, err := Open(dir)
		require.NoError(t, err)
		defer func() { require.NoError(t, s.Close()) }()
		info, err := os.Stat(filepath.Join(dir, "projects"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	})
	t.Run("a restrictive umask leaves the modes exact", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "romeu")
		old := syscall.Umask(0o277)
		defer syscall.Umask(old)
		s, err := Open(dir)
		require.NoError(t, err)
		defer func() { require.NoError(t, s.Close()) }()
		require.NoError(t, writeFile(s.projects, "a.json", []byte("x")))
		for _, p := range []string{s.Dir(), filepath.Join(s.Dir(), "projects")} {
			info, err := os.Stat(p)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o700), info.Mode().Perm(), p)
		}
		info, err := os.Stat(filepath.Join(s.Dir(), "projects", "a.json"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})
}
