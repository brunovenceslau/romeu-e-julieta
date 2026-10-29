// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package raises.
var (

	// DriftDetected is the error drift-detected (RJ-303): step 7 found a
	// live derived file that differs from the current render.json
	// (REQ-0342).
	DriftDetected = cli.Row{
		ID:          "RJ-303",
		Slug:        "drift-detected",
		Exit:        4,
		Fix:         "decide: move the edit into the spec, or run romeu sync --overwrite-drift <name>, which loses it",
		RecoveredIn: "none: the edited file is left as it is, and the hint chooses what replaces it",
	}

	// JulietaNotEmbedded is the error julieta-not-embedded (RJ-312): a
	// romeu built without tools/release build holds a placeholder for
	// julieta, so it refuses sync before it writes (02 2.1, REQ-0130).
	JulietaNotEmbedded = cli.Row{
		ID:          "RJ-312",
		Slug:        "julieta-not-embedded",
		Exit:        2,
		Fix:         "decide: install a released romeu, or build one with tools/release build",
		RecoveredIn: "none: nothing was written, and a romeu that embeds julieta runs the command",
	}

	// ManifestSize is the error manifest-size (RJ-313): a project's
	// encoded manifest is over 32 KiB, which step 6 checks before gate 2
	// (03 3.6, REQ-0216).
	ManifestSize = cli.Row{
		ID:          "RJ-313",
		Slug:        "manifest-size",
		Exit:        2,
		Fix:         "decide: shorten the spec named in the details, then run romeu sync <name>",
		RecoveredIn: "none: nothing was promoted, and the sync of the hint renders again",
	}

	// SecretUnbound is the error secret-unbound (RJ-315): a spec secret
	// has no <name>@<project> entry in the host settings, so step 1 stops
	// (04 4.2, REQ-0335).
	SecretUnbound = cli.Row{
		ID:          "RJ-315",
		Slug:        "secret-unbound",
		Exit:        2,
		Fix:         "decide: paste the entry the details print into the secrets of the host settings, then run romeu sync <name>",
		RecoveredIn: "none: nothing was written, and the sync of the hint reads the settings again",
	}
)
