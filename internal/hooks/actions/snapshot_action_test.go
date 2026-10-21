// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package actions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSnapshotRunsAfterTheThreeHooks (04 4.3, REQ-0408; 08 8.4,
// REQ-0686): post-commit, post-rewrite and post-merge each have the
// action snapshot, whose failure continues the chain (REQ-0409), and no
// other hook has it.
func TestSnapshotRunsAfterTheThreeHooks(t *testing.T) {
	for _, hook := range []string{"post-commit", "post-rewrite", "post-merge"} {
		got := For(hook)
		require.Len(t, got, 1, hook)
		assert.Equal(t, "snapshot", got[0].Name, hook)
		assert.True(t, got[0].Continues, hook)
	}
	for _, hook := range []string{"pre-commit", "post-checkout", "pre-push", "reference-transaction"} {
		for _, a := range For(hook) {
			assert.NotEqual(t, "snapshot", a.Name, hook)
		}
	}
}

// TestSnapshotActionRunsTheHandler: the action runs what the package of
// julieta snapshot set with HandleSnapshot, with the hook's input; with
// none set it fails, so a binary that does not link the command says
// so rather than skip the snapshot unseen.
func TestSnapshotActionRunsTheHandler(t *testing.T) {
	a := For("post-merge")[0]
	seam(t, &snapshotHandler, nil)
	require.Error(t, a.Run(t.Context(), Input{Hook: "post-merge"}))

	var got Input
	HandleSnapshot(func(_ context.Context, in Input) error { got = in; return nil })
	require.NoError(t, a.Run(t.Context(), Input{Hook: "post-merge", Args: []string{"0"}, Dir: "/r"}))
	assert.Equal(t, Input{Hook: "post-merge", Args: []string{"0"}, Dir: "/r"}, got)
}
