// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package actions

import (
	"context"
	"errors"
)

// snapshotHandler is what the snapshot action runs: the snapshot of the
// repo of the hook's directory, which needs the manifest, julieta state
// and the clock, so the package of julieta snapshot sets it
// (HandleSnapshot), since this package cannot import that one (it
// imports internal/cmd/julieta/setup, which imports internal/hooks).
var snapshotHandler func(ctx context.Context, in Input) error

// HandleSnapshot sets the function the snapshot actions of post-commit,
// post-rewrite and post-merge run (08 8.4, REQ-0686). It is called once,
// during initialization, by internal/cmd/julieta/snapshot, the way an
// action registers itself, so it is written then and read-only after.
// It returns f, so that a file can write var _ = HandleSnapshot(...).
func HandleSnapshot(f func(ctx context.Context, in Input) error) func(ctx context.Context, in Input) error {
	snapshotHandler = f
	return f
}

// snapshotAction is julieta's action after a commit, a rewrite and a
// merge: the snapshot, with no debounce (REQ-0686), whose failure is
// recorded and does not stop the chain (04 4.3, REQ-0409; REQ-0691).
var snapshotAction = Action{Name: "snapshot", Run: runSnapshot, Continues: true}

// runSnapshot runs the handler HandleSnapshot set, and fails when none
// is set, so a binary that does not link julieta snapshot says so.
func runSnapshot(ctx context.Context, in Input) error {
	if snapshotHandler == nil {
		return errors.New("julieta snapshot is not linked into this binary, so no snapshot was taken")
	}
	return snapshotHandler(ctx, in)
}
