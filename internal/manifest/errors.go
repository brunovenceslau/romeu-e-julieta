// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package manifest

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package raises.
var (
	// FormatNewer is the error format-newer (RJ-311): the document's
	// schema field names a version of the manifest newer than Version
	// (03, opening, REQ-0163).
	FormatNewer = cli.Row{
		ID:          "RJ-311",
		Slug:        "format-newer",
		Exit:        2,
		Fix:         "decide: install the release that wrote it",
		RecoveredIn: "none: the release that wrote the file reads it",
	}

	// ManifestProtocol is the error manifest-protocol (RJ-342): the
	// manifest's protocol is outside N and N-1 (03 3.6, REQ-0219,
	// REQ-0440). romeu run passes a manifest of its own protocol.
	ManifestProtocol = cli.Row{
		ID:          "RJ-342",
		Slug:        "manifest-protocol",
		Exit:        2,
		Fix:         "decide: run romeu run <name> on the host, which passes a manifest of the current protocol",
		RecoveredIn: "none: the run of the hint replaces the manifest",
	}
)
