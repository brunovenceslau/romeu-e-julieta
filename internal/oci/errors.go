// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// The rows of the error table (04 4.4) that this package declares. The
// other row it raises, upstream-shape (RJ-310), is declared once, in
// internal/cli, since every decoder of an upstream output raises it.
var (
	// RegistryUnreachable is the error registry-unreachable (RJ-322): a
	// registry did not answer, answered with a status that is not a
	// manifest, or asks for credentials, which the product does not have
	// (REQ-0433, REQ-0210).
	RegistryUnreachable = cli.Row{
		ID:          "RJ-322",
		Slug:        "registry-unreachable",
		Exit:        1,
		Fix:         "decide: check the network and the registry named in the details, then run the command again; a private registry is not supported in v1",
		RecoveredIn: "none: nothing was changed, and the rerun reads the registry again",
	}
)
