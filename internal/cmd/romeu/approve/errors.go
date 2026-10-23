// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package approve

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package raises.
var (
	// ToolchainApprovalRequired is the error toolchain-approval-required
	// (RJ-302): the toolchain this machine runs is not the one gate 1
	// recorded, or none was recorded (01 1.4, REQ-0062). Check returns
	// it; the preflight (step 1) and sync stop with it.
	ToolchainApprovalRequired = cli.Row{
		ID:          "RJ-302",
		Slug:        "toolchain-approval-required",
		Exit:        3,
		Fix:         "romeu approve --toolchain",
		RecoveredIn: "none: nothing was changed, and the acknowledgement of the hint records the toolchain",
	}
)
