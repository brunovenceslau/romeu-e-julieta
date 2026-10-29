// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package salvage

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package raises: the
// host half of a salvage (08 8.5), which romeu salvage, rm, recreate
// and retire run.
var (
	// SalvagePath is the error salvage-path (RJ-307): a path of a
	// salvage manifest breaks the rule of 03 3.11 (REQ-0252): it is not
	// relative and clean, holds a ".." element or a symbolic link, or
	// names what is not a regular file below the salvage dir.
	SalvagePath = cli.Row{
		ID:          "RJ-307",
		Slug:        "salvage-path",
		Exit:        5,
		Fix:         "decide: rerun; a path that persists names a julieta or a sandbox process that wrote into the salvage dir, report it as an issue",
		RecoveredIn: "none: no ref was imported from that manifest, and the rerun salvages into a new dir",
	}

	// SalvageIDMismatch is the error salvage-id-mismatch (RJ-339): the
	// manifest's salvageId is not the id romeu passed (03 3.11,
	// REQ-0248). It is not a skipped reason, so --accept-loss does not
	// cover it (REQ-0250).
	SalvageIDMismatch = cli.Row{
		ID:          "RJ-339",
		Slug:        "salvage-id-mismatch",
		Exit:        5,
		Fix:         "decide: rerun; a mismatch that persists names a julieta that wrote another salvage's manifest, report it as an issue",
		RecoveredIn: "none: no ref was imported from that manifest, and the rerun salvages under a new id",
	}
)
