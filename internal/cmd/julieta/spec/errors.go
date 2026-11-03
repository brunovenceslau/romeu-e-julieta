// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package raises.
var (
	// FetchFailed is the error fetch-failed (RJ-360): with --catalog,
	// the mise.lock of a repo cannot be read from its origin, so the
	// command stops rather than pass on a lock it did not read. The
	// details name the repo URL and the ref. The same slug is the
	// warning of julieta setup for a fetch that failed (04 4.1,
	// Warnings).
	FetchFailed = cli.Row{
		ID:          "RJ-360",
		Slug:        "fetch-failed",
		Exit:        1,
		Fix:         "decide: check the network and the origin URL, then run the command again",
		RecoveredIn: "none: nothing was written, and the rerun reads the origin again",
	}
)
