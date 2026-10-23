// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package declares. The
// row it raises for a listing it cannot read, upstream-shape (RJ-310),
// is declared once, in internal/cli.
var (
	// UnknownSandbox is the error unknown-sandbox (RJ-203): step 6 found
	// a sandbox of the project's name that romeu has no open generation
	// for, or one whose workspace is not $ROMEU_ROOT/<name>-env/<primary>
	// (I26, REQ-0076, REQ-0511). The hint is filled with the project's
	// name and primary directory.
	UnknownSandbox = cli.Row{
		ID:          "RJ-203",
		Slug:        "unknown-sandbox",
		Exit:        2,
		Fix:         "decide: confirm that <name> is this project's sandbox (its workspace is $ROMEU_ROOT/<name>-env/<primary>), then romeu adopt <name>",
		RecoveredIn: "none: nothing was changed, and adopting the sandbox records it",
	}

	// SbxGlobalSecret is the error sbx-global-secret (RJ-329): step 5
	// found an sbx secret whose scope is not one sandbox for a service
	// the project uses (I22, REQ-0371, REQ-0506). The hint is filled with
	// the service and the project's name.
	SbxGlobalSecret = cli.Row{
		ID:          "RJ-329",
		Slug:        "sbx-global-secret",
		Exit:        2,
		Fix:         "decide: bind the service <service> to this sandbox only with sbx secret set <service> --sandbox <name> and the --command that uses it, then remove the global secret of that service with sbx secret rm",
		RecoveredIn: "none: nothing was changed, and the next run reads the secret listing again",
	}

	// SbxBroadAllow is the error sbx-broad-allow (RJ-330): step 5 found a
	// global allow rule of ** or another broad wildcard, or a permissive
	// default network policy (I22, REQ-0372, REQ-0506). The details name
	// the rule.
	SbxBroadAllow = cli.Row{
		ID:          "RJ-330",
		Slug:        "sbx-broad-allow",
		Exit:        2,
		Fix:         "decide: remove the rule named in the details with sbx policy rm, or set the default network policy to deny, and allow only the domains of the project per sandbox",
		RecoveredIn: "none: nothing was changed, and the next run reads the policy listing again",
	}

	// HomeSymlink is the error home-symlink (RJ-331): step 5 found a
	// symbolic link under $HOME or $HOME/.config that resolves into
	// $ROMEU_ROOT (I23, REQ-0378, REQ-0507).
	HomeSymlink = cli.Row{
		ID:          "RJ-331",
		Slug:        "home-symlink",
		Exit:        2,
		Fix:         "decide: remove the link named in the details, or point it at a place outside $ROMEU_ROOT",
		RecoveredIn: "none: nothing was changed, and the next run looks at the links again",
	}

	// AutoTrust is the error auto-trust (RJ-332): step 5 found a mise
	// trusted_config_paths entry or a direnv allow entry that covers
	// $ROMEU_ROOT (I23, REQ-0379, REQ-0507).
	AutoTrust = cli.Row{
		ID:          "RJ-332",
		Slug:        "auto-trust",
		Exit:        2,
		Fix:         "decide: remove the trust entry named in the details from the mise or direnv settings it comes from",
		RecoveredIn: "none: nothing was changed, and the next run reads the trust settings again",
	}

	// VSCodeTrust is the error vscode-trust (RJ-333): step 5 found that
	// VS Code workspace trust is turned off, or that $ROMEU_ROOT, a
	// parent of it or a folder under it is a trusted folder (I14, I23,
	// REQ-0380, REQ-0507).
	VSCodeTrust = cli.Row{
		ID:          "RJ-333",
		Slug:        "vscode-trust",
		Exit:        2,
		Fix:         "decide: turn security.workspace.trust.enabled back on in the VS Code settings, or remove the trusted folder named in the details",
		RecoveredIn: "none: nothing was changed, and the next run reads the VS Code settings again",
	}

	// RemovalPending is the error removal-pending (RJ-336): step 6 found
	// a generation in the state removing with its sandbox present, and
	// the command is not rm, recreate or retire (REQ-0436, REQ-0076).
	RemovalPending = cli.Row{
		ID:          "RJ-336",
		Slug:        "removal-pending",
		Exit:        2,
		Fix:         "decide: run romeu rm <name>, which finishes the removal an earlier command began",
		RecoveredIn: "none: the hint finishes the removal that an earlier command began",
	}
)
