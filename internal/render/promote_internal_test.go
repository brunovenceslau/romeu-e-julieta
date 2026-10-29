// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
)

// Step 2 verifies a live kit dir again, whatever preflight found: a dir
// swapped after preflight is refused, not taken as the approved kit.
func TestEnsureKitDirVerifiesAnExistingDirAgain(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })
	files := []gate.File{{Path: "k.yaml", Mode: gate.ModeRegular, Data: []byte("name: k\n")}}
	want, err := gate.KitTreeDigest(files)
	require.NoError(t, err)
	p := path.Join(LiveKitsDir, "k-"+want.String()[:12])
	require.NoError(t, ensureKitDir(root, p, want.String(), files))
	require.NoError(t, ensureKitDir(root, p, want.String(), files), "a dir that verifies is left as it is")

	require.NoError(t, root.WriteFile(path.Join(p, "k.yaml"), []byte("swapped\n"), 0o600))
	require.ErrorIs(t, ensureKitDir(root, p, want.String(), files), ErrKitDirDiffers)
}

// writeWorkspace holds each name to the two workspace files itself, not
// only through preflight.
func TestWriteWorkspaceRefusesAnotherName(t *testing.T) {
	l := Live{Root: t.TempDir(), Workspace: []WorkspaceFile{{Name: "../escape", Data: []byte("x")}}}
	err := l.writeWorkspace()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a workspace file")
}
