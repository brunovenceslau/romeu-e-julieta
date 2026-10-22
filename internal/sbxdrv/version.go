// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import "github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"

var _ = registerBuilder("version", func() []Call { return []Call{Version()} })

// Version is sbx version: a read (timeout class sbx-read), captured.
// Its output is read by ParseVersion; gate 1, the floor check and
// romeu version --json read it.
func Version() Call {
	return mustCall((&argvBuilder{}).lit("version").call("version", procgroup.SbxRead, Captured))
}
