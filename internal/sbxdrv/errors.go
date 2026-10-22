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
