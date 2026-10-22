// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

func TestCandidate_Apply_illegal(t *testing.T) {
	root := filepath.Join(t.TempDir(), "candidates")
	c, err := RenderCandidate(root, testSeams())
	require.NoError(t, err)
	_, err = c.Apply(CandidatePromotionDone, nil, Approval{})
	require.Error(t, err)
	assert.Equal(t, "RJ-350", cli.Of(err).ID)
	assert.Equal(t, CandidateRendered, c.State(), "the state is as it was")
	assert.DirExists(t, c.Path())

	_, err = c.Apply(CandidateApprove, nil, Approval{CandidateID: c.ID()})
	require.Error(t, err, "an approval needs a project record")
	assert.Equal(t, CandidateRendered, c.State())
}

func TestRenderCandidate_errors(t *testing.T) {
	t.Run("a source with no entropy", func(t *testing.T) {
		_, err := RenderCandidate(filepath.Join(t.TempDir(), "candidates"),
			Seams{Now: func() time.Time { return testNow }})
		require.ErrorIs(t, err, canon.ErrULIDSource)
	})
	t.Run("a candidates path that is a file", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(file, nil, 0o600))
		_, err := RenderCandidate(file, testSeams())
		require.Error(t, err)
	})
	t.Run("a candidates dir that cannot be made", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(file, nil, 0o600))
		_, err := RenderCandidate(filepath.Join(file, "candidates"), testSeams())
		require.Error(t, err)
	})
}

func TestRenderCandidate_staysInItsDirectory(t *testing.T) {
	t.Run("a candidates path that is a symbolic link is refused, and its target is untouched", func(t *testing.T) {
		base := t.TempDir()
		victim := filepath.Join(base, "victim")
		require.NoError(t, os.MkdirAll(filepath.Join(victim, "01j9z00000000000000000abcd"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(victim, "01j9z00000000000000000abcd", "keep"), []byte("x"), 0o600))
		require.NoError(t, os.Symlink(victim, filepath.Join(base, "candidates")))
		_, err := RenderCandidate(filepath.Join(base, "candidates"), testSeams())
		require.Error(t, err)
		assert.FileExists(t, filepath.Join(victim, "01j9z00000000000000000abcd", "keep"))
		entries, err := os.ReadDir(victim)
		require.NoError(t, err)
		assert.Len(t, entries, 1, "nothing was made in the target")
	})
	t.Run("only entries named by a ULID are deleted", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "candidates")
		require.NoError(t, os.MkdirAll(filepath.Join(root, "01j9z00000000000000000abcd"), 0o700))
		require.NoError(t, os.MkdirAll(filepath.Join(root, "notes"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(root, "README"), []byte("x"), 0o600))
		c, err := RenderCandidate(root, testSeams())
		require.NoError(t, err)
		assert.NoDirExists(t, filepath.Join(root, "01j9z00000000000000000abcd"))
		assert.DirExists(t, filepath.Join(root, "notes"))
		assert.FileExists(t, filepath.Join(root, "README"))
		assert.DirExists(t, c.Path())
	})
}
