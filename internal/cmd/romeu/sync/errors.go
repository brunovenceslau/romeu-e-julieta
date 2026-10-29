// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package raises.
var (
	// ProjectApprovalRequired is the error project-approval-required
	// (RJ-301): gate 2 found a widening digest that differs from the
	// approval with no terminal to ask on, so the candidate was deleted
	// and its digest printed (01 1.6, REQ-1210). Step 8 of sync raises
	// it; preflight step 4 raises the same row once a command that runs
	// the preflight lands, and the row moves to the package both share
	// then.
	ProjectApprovalRequired = cli.Row{
		ID:          "RJ-301",
		Slug:        "project-approval-required",
		Exit:        3,
		Fix:         "romeu sync <name>",
		RecoveredIn: "none: the candidate was deleted and nothing was promoted, and the sync of the hint, on a terminal, asks gate 2",
	}

	// InterruptedPromotionAbandoned is the error
	// interrupted-promotion-abandoned (RJ-304): promotion recovery found
	// a promotion whose candidate no longer verifies and wrote render.json
	// back (01 1.6, REQ-0087). Step 1 of sync raises it; every mutating
	// command runs the same recovery once it lands, and the row moves to
	// the package they share then.
	InterruptedPromotionAbandoned = cli.Row{
		ID:          "RJ-304",
		Slug:        "interrupted-promotion-abandoned",
		Exit:        4,
		Fix:         "romeu sync <name>",
		RecoveredIn: "none: the live files are those of the last promotion that completed, and the sync of the hint renders a fresh candidate",
	}

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

	// CandidateDeclined is the error candidate-declined (RJ-358): the
	// operator answered gate 2 on a terminal with anything but y, so the
	// candidate dir was deleted and the live files are unchanged (01 1.6,
	// REQ-1209). It fails the command, as gate 1's decline does
	// (REQ-1276), so a command chained after it does not run; the targets
	// after the declined one still meet gate 2 (decided on 2026-10-29).
	CandidateDeclined = cli.Row{
		ID:          "RJ-358",
		Slug:        "candidate-declined",
		Exit:        1,
		Fix:         "romeu sync <name>",
		RecoveredIn: "none: the candidate was deleted and the live files are unchanged, and the sync of the hint renders again and asks",
	}
)
