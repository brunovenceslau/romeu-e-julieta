// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"context"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

var _ = registerBuilder("version", func(Fixture) []Call { return []Call{Version()} })

// Version is sbx version: a read (timeout class sbx-read), captured.
// Its output is read by ParseVersion; gate 1, the floor check and
// romeu version --json read it.
func Version() Call {
	return mustCall((&argvBuilder{}).lit("version").call("version", procgroup.SbxRead, Captured))
}

// Version runs sbx version and reads its output. A call that fails is
// a *CallError; an output that is not a version is the error
// upstream-shape (RJ-310), with the recorded versions in its details.
func (r *Runner) Version(ctx context.Context, o Opts) (Release, error) {
	res, err := r.Run(ctx, Version(), o)
	if err != nil {
		return Release{}, err
	}
	return ParseVersion(res.Stdout)
}

// RequireFloor is the sbx floor check before gate 1: it reads the
// version of sbx and refuses one below Floor with the error sbx-floor
// (RJ-343, exit 2). It runs before any other sbx call of a command, so
// that an sbx too old to be driven is never driven.
func (r *Runner) RequireFloor(ctx context.Context, o Opts) (Release, error) {
	v, err := r.Version(ctx, o)
	if err != nil {
		return Release{}, err
	}
	return v, CheckFloor(v)
}
