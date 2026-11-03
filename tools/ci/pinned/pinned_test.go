// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package pinned

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// root is the root of the repository, three directories above this
// package's.
func root(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	return abs
}

// TestWhichResolvesAPinnedTool resolves git-cliff, which mise.toml
// pins: an absolute path to an existing file, and a name that is not
// pinned is refused before mise starts.
func TestWhichResolvesAPinnedTool(t *testing.T) {
	path, err := Which(t.Context(), root(t), "git-cliff")
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(path), path)
	assert.FileExists(t, path)

	_, err = Which(t.Context(), root(t), "not-a-tool")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not one of the pinned tools")
}
