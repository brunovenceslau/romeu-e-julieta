// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"context"

	"github.com/brunovenceslau/romeu-e-julieta/internal/preflight"
)

// The rule signing-socket (06 6.4, I34, REQ-0600): sync refuses to
// render the product kit git-ssh-sign for a target unless
// signing.agentSocket of the host settings is set, reachable and holds
// exactly one key, the kit's signingKey; the error is signing-socket
// (RJ-204, exit 2). It is the check of step 5 of the preflight and of
// doctor's row, preflight.CheckSigning, so the three agree (REQ-0601).
// It runs before gate 2 and before anything is promoted.
var _ = register(rule{name: "signing-socket", check: checkSigningSocket})

// checkSigningSocket refuses the first target that uses the kit and
// whose socket fails the check.
func checkSigningSocket(ctx context.Context, r *run) error {
	for _, p := range r.targets {
		s := preflight.SigningOf(&r.settings, p.Kits)
		if s == nil {
			continue
		}
		if err := preflight.CheckSigning(ctx, p.Name, *s); err != nil {
			return err
		}
	}
	return nil
}
