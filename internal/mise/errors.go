// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package raises.
var (
	// LockMissing is the error lock-missing (RJ-340): a repo has a
	// mise.toml and no mise.lock (07 7.3, REQ-0438). julieta install
	// raises it; the hint is julieta lock --repo <dir>.
	LockMissing = cli.Row{
		ID:          "RJ-340",
		Slug:        "lock-missing",
		Exit:        2,
		Fix:         "julieta lock --repo <dir>",
		RecoveredIn: "none: the hint locks the repo and the install runs again",
	}

	// InstallBlocked is the error install-blocked (RJ-341): a download
	// of mise install that the sandbox's egress policy blocked (04 4.3,
	// REQ-0439). Its details name the tool, the host, the sbx command
	// that approves the host once for this sandbox, that the approval
	// is lost at recreate, the durable fix, and whether the host is in
	// the egress set of the manifest.
	InstallBlocked = cli.Row{
		ID:          "RJ-341",
		Slug:        "install-blocked",
		Exit:        1,
		Fix:         "decide: approve the host once for this sandbox with the sbx command named, or add the host to the spec's egress and run gate 2",
		RecoveredIn: "none: the details name the approval and the spec edit that fix it",
	}

	// MiseFailed is the error mise-failed (RJ-351): a mise that exited
	// non-zero, or was ended by a signal, and is neither a blocked
	// download (install-blocked) nor past its timeout (subprocess-timeout).
	// julieta install and julieta lock raise it; the details hold the
	// argv, the status and the start of mise's output (04 4.1, Error ids,
	// item 7).
	MiseFailed = cli.Row{
		ID:          "RJ-351",
		Slug:        "mise-failed",
		Exit:        1,
		Fix:         "decide: read the stderr in the details, fix what mise reports, then run the command again",
		RecoveredIn: "none: the details name what mise reported",
	}
)
