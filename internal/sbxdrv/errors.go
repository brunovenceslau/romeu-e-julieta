// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package sbxdrv

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package raises.
var (
	// SbxUnknown is the error sbx-unknown (RJ-308): a call that tells
	// romeu whether a sandbox is absent, sbx ls --json, exited non-zero,
	// so nothing is known and nothing changes (01 1.6, REQ-0102; 03,
	// opening, REQ-0171).
	SbxUnknown = cli.Row{
		ID:          "RJ-308",
		Slug:        "sbx-unknown",
		Exit:        1,
		Fix:         "decide: run sbx ls --json yourself; when the daemon is down, start it with sbx daemon start, then run the command again",
		RecoveredIn: "none: nothing was changed, and the rerun reads sbx again",
	}

	// UpstreamShape is the error upstream-shape (RJ-310): an output of
	// sbx, a tool the product does not own, does not decode, or lacks a
	// field romeu reads or has it mistyped. No parser returns an absent
	// sandbox or an empty list for a shape it did not recognize (03,
	// opening, REQ-0170; 01 1.4, REQ-0064).
	UpstreamShape = cli.Row{
		ID:          "RJ-310",
		Slug:        "upstream-shape",
		Exit:        2,
		Fix:         "decide: run sbx version and compare it with the recorded versions in the details; an sbx outside them is reported as an issue with the details",
		RecoveredIn: "none: the command stops before it changes any state, and a release that records the new shape reads it",
	}

	// SbxFloor is the error sbx-floor (RJ-343): the sbx on the host is
	// below the floor of 12 12.1, checked before gate 1 (REQ-0441).
	SbxFloor = cli.Row{
		ID:          "RJ-343",
		Slug:        "sbx-floor",
		Exit:        2,
		Fix:         "decide: install sbx at or above the floor of 12 12.1",
		RecoveredIn: "none: the command stops before it runs any other sbx call",
	}
)
