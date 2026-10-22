// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package state

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package raises.
var (
	// Locked is the error locked (RJ-101): another romeu holds
	// romeu.lock (04 4.1, Locking, REQ-0283).
	Locked = cli.Row{
		ID:          "RJ-101",
		Slug:        "locked",
		Exit:        1,
		Fix:         "decide: wait for the other romeu to end, then run the command again",
		RecoveredIn: "none: the lock is released when the other romeu ends, and the rerun takes it",
	}

	// RecordUnreadable is the error record-unreadable (RJ-334): a
	// record of a format this build knows does not decode. The file is
	// never overwritten (03, opening, REQ-0165).
	RecordUnreadable = cli.Row{
		ID:          "RJ-334",
		Slug:        "record-unreadable",
		Exit:        2,
		Fix:         "decide: restore the file named in the details from a backup, and run romeu salvage --from-host <name> first to save the work of the project",
		RecoveredIn: "none: the restore of the hint replaces the file, and romeu never wrote over it",
	}

	// IllegalTransition is the error illegal-transition (RJ-350): a
	// (from, event) pair that no row of a state machine has (01 1.6,
	// REQ-0079).
	IllegalTransition = cli.Row{
		ID:          "RJ-350",
		Slug:        "illegal-transition",
		Exit:        2,
		Fix:         "decide: run romeu status to read the state the record is in, and report the command that raised the event as an issue",
		RecoveredIn: "none: the transition is refused and the record is left as it was",
	}
)
