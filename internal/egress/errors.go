// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package egress

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package raises. It also
// raises lock-unparsed (RJ-325) and lock-missing (RJ-340), which
// internal/cli declares since both binaries raise them.
var (
	// LockHostInvalid is the error lock-host-invalid (RJ-326): a host
	// of an artifact URL in a mise.lock fails the egress domain rule of
	// 03 3.1 (07 7.5, step 3). The message names the repo and the lock
	// entry.
	LockHostInvalid = cli.Row{
		ID:          "RJ-326",
		Slug:        "lock-host-invalid",
		Exit:        2,
		Fix:         "decide: pin a release of the tool that is served from a host name the egress domain rule accepts, then run julieta lock --repo <dir>",
		RecoveredIn: "none: the message names the entry that holds the host",
	}

	// CatalogUnknownTool is the error catalog-unknown-tool (RJ-327): an
	// entry of a mise.lock names a backend:tool that is in no entry of the
	// catalog and in no egress.tools entry of the spec (07 7.5, step 2).
	// The message names the tools and the two fixes, the details each
	// tool and its repo.
	CatalogUnknownTool = cli.Row{
		ID:          "RJ-327",
		Slug:        "catalog-unknown-tool",
		Exit:        2,
		Fix:         "decide: open a pull request that adds the tool to the egress catalog, or declare it in egress.tools of the project spec, which is gated",
		RecoveredIn: "none: the catalog entry ships in a release, and the spec entry takes effect at the next sync",
	}
)
