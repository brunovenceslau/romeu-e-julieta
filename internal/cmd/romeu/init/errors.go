// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package initcmd

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package raises.
var (
	// ConfigCloneFailed is the error config-clone-failed (RJ-356): git
	// could not clone the config repo, for a URL that names no
	// repository, an access the origin refuses, or a private repository
	// and no --git-credential-helper. The details hold the start of
	// git's output, escaped (04 4.1, Error ids, item 7).
	ConfigCloneFailed = cli.Row{
		ID:          "RJ-356",
		Slug:        "config-clone-failed",
		Exit:        1,
		Fix:         "decide: check the URL and your access; a private config repo needs --git-credential-helper",
		RecoveredIn: "none: the details hold what git reported, and nothing was written",
	}
)
