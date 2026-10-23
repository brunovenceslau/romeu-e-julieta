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

	// ToolchainDeclined is the error toolchain-declined (RJ-355): the
	// operator answered gate 1 with anything but y, so the toolchain is
	// not acknowledged and the command fails, which stops a command line
	// chained after it (decided on 2026-10-23).
	ToolchainDeclined = cli.Row{
		ID:          "RJ-355",
		Slug:        "toolchain-declined",
		Exit:        1,
		Fix:         "romeu approve --toolchain",
		RecoveredIn: "none: toolchain.json is unchanged, and the rerun of the hint asks again",
	}
)
