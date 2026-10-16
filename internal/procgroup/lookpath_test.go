// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLookPath checks that LookPath finds a program the way Run does,
// on this process's PATH, and that a name not there is ErrNotFound.
func TestLookPath(t *testing.T) {
	dir := t.TempDir()
	program := filepath.Join(dir, "procgroup-lookpath-probe")
	require.NoError(t, os.WriteFile(program, []byte("#!/bin/sh\nexit 0\n"), 0o700))
	t.Setenv("PATH", dir)

	t.Run("a program on PATH is found by its absolute path", func(t *testing.T) {
		got, err := LookPath("procgroup-lookpath-probe")
		require.NoError(t, err)
		assert.Equal(t, program, got)
	})
	t.Run("a name that is not on PATH is ErrNotFound", func(t *testing.T) {
		_, err := LookPath("procgroup-lookpath-absent")
		require.ErrorIs(t, err, ErrNotFound)
	})
	t.Run("a path is taken as it is", func(t *testing.T) {
		got, err := LookPath(program)
		require.NoError(t, err)
		assert.Equal(t, program, got)
	})
}
