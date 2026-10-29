// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package raises.
var (
	// ManifestSize is the error manifest-size (RJ-313): a project's
	// encoded manifest is over 32 KiB, which step 6 checks before gate 2
	// (03 3.6, REQ-0216).
	ManifestSize = cli.Row{
		ID:          "RJ-313",
		Slug:        "manifest-size",
		Exit:        2,
		Fix:         "decide: shorten what the project the details name puts into its manifest, under 32 KiB, then run romeu sync <name>",
		RecoveredIn: "none: nothing was promoted, and the sync of the hint renders again",
	}

	// SecretUnbound is the error secret-unbound (RJ-315): a spec secret
	// of a target has no <secret>@<project> entry in the host settings,
	// so the end of step 3 stops, before step 4 writes (04 4.2,
	// REQ-0335). The hint is filled with the secret's
	// name and the project, the entry to paste.
	SecretUnbound = cli.Row{
		ID:          "RJ-315",
		Slug:        "secret-unbound",
		Exit:        2,
		Fix:         "decide: add the entry <secret>@<name> to the secrets of the host settings, then run romeu sync <name>",
		RecoveredIn: "none: nothing was rendered or promoted, and adding the entry binds the secret",
	}
)
