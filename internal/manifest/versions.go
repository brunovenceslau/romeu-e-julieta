// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package manifest

// The versions of the manifest (03 3.6). This file is on the contracts
// surface of .github/ask-first.yaml: a change to either number changes
// what romeu and julieta accept from each other.
const (
	// Version is the version of the format manifest, the n of the schema
	// field "manifest/v<n>" (REQ-0161). A reader refuses a newer one with
	// format-newer (REQ-0163).
	Version = 1

	// Protocol is the romeu/julieta contract number N this build speaks
	// (REQ-0218). julieta accepts a manifest of protocol N or N-1 and
	// refuses any other (REQ-0219).
	Protocol = 1

	// FirstProtocol is the first protocol there was. No protocol below
	// it exists, so a manifest that names one, or none (the zero value),
	// is refused even where N-1 would reach it.
	FirstProtocol = 1
)

// acceptsProtocol reports whether julieta, speaking the protocol n,
// accepts a manifest of protocol p: p is n or n-1 (REQ-0219), and not
// below FirstProtocol.
func acceptsProtocol(p, n int) bool {
	return p >= FirstProtocol && (p == n || p == n-1)
}
