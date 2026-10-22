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
