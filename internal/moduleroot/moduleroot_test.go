// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package moduleroot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFind checks that Find returns the nearest directory at or above
// dir that holds go.mod: dir itself, a parent, the nearest of two, and
// an error when none does.
func TestFind(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module m\n"), 0o600))
	deep := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(deep, 0o755))

	got, err := Find(root)
	require.NoError(t, err)
	assert.Equal(t, root, got, "dir itself")

	got, err = Find(deep)
	require.NoError(t, err)
	assert.Equal(t, root, got, "a parent")

	nested := filepath.Join(root, "a")
	require.NoError(t, os.WriteFile(filepath.Join(nested, "go.mod"), []byte("module n\n"), 0o600))
	got, err = Find(deep)
	require.NoError(t, err)
	assert.Equal(t, nested, got, "the nearest")
}

// TestFindWithoutGoMod checks the error when no directory above holds
// go.mod. It fails, rather than skips, if one does: a go.mod above the
// temporary directory of the machine is a fault of the machine to see.
func TestFindWithoutGoMod(t *testing.T) {
	_, err := Find(t.TempDir())
	require.Error(t, err, "a go.mod above the temporary directory")
	assert.ErrorContains(t, err, "no go.mod")
}
