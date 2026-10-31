// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package status

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package raises.
var (
	// StatusIncomplete is the error status-incomplete (RJ-359): a read of
	// the report failed, so the report was printed with each failed read
	// in its project's block, or at its head for a read of the machine's,
	// and the command fails (04 4.2, REQ-0329). It
	// is also the id of a failed read that carries no row of its own,
	// such as a record that cannot be opened; a read with a row of its
	// own (sbx-unknown, record-unreadable) keeps it. Declared on
	// 2026-10-30 with T048: no row of 04 4.4 had exit 1 and this meaning.
	StatusIncomplete = cli.Row{
		ID:          "RJ-359",
		Slug:        "status-incomplete",
		Exit:        1,
		Fix:         "decide: read the error of each failed read in its project's block and follow its own hint, then run romeu status again",
		RecoveredIn: "none: status is read only, so it changed nothing",
	}
)
