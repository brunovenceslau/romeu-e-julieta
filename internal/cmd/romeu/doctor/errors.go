// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package doctor

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package raises. A check
// that ran and failed is the error check-findings (RJ-328), declared once
// in internal/cli.
var (
	// DoctorIncomplete is the error doctor-incomplete (RJ-360): a check
	// could not run, so the report is incomplete; it was printed with
	// each such check as an error, by its own id, and every failed check
	// (04 4.2, REQ-0331). Declared on 2026-10-30 with T049: no row of 04
	// 4.4 had exit 1 and this meaning.
	DoctorIncomplete = cli.Row{
		ID:          "RJ-360",
		Slug:        "doctor-incomplete",
		Exit:        1,
		Fix:         "decide: read the error of each check the report marks error and follow its own hint, then run romeu doctor again",
		RecoveredIn: "none: doctor is read only, so it changed nothing",
	}
)
