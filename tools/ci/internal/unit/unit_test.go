// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// TestSteps checks that the unit step names the directories of its row
// that exist, and leaves out one that does not, or that is a file.
func TestSteps(t *testing.T) {
	root := t.TempDir()
	argv := func() []string {
		steps := Steps(t.Context(), check.Plan{Root: root, Tools: check.Tools{GoDir: "/pinned/bin"}})
		require.Len(t, steps, 1)
		assert.Equal(t, "unit", steps[0].Name)
		assert.Equal(t, check.StepEnv("/pinned/bin"), steps[0].Environ)
		return steps[0].Argv
	}
	base := []string{"/pinned/bin/go", "test", "-count=1"}
	assert.Equal(t, base, argv(), "neither directory")
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal"), nil, 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(root, "tools"), 0o755))
	assert.Equal(t, append(base, "./tools/..."), argv(), "internal is a file")
	require.NoError(t, os.Remove(filepath.Join(root, "internal")))
	require.NoError(t, os.Mkdir(filepath.Join(root, "internal"), 0o755))
	assert.Equal(t, append(base, "./internal/...", "./tools/..."), argv(), "both")
}
