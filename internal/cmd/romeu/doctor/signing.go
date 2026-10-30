// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/preflight"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// slugSigningSocket is the check signing-socket, marked pre in 04 4.2
// (REQ-0384, I34), which shares its slug with the error RJ-204.
const slugSigningSocket = "signing-socket"

// The check signing-socket: for each project whose approved widening set
// holds the product kit git-ssh-sign, signing.agentSocket of the host
// settings is set, reachable and holds exactly one key, the kit's
// signingKey. It runs preflight.CheckSigning, the check of step 5 and of
// sync's rule (REQ-0365, REQ-0601), once per such project: doctor's
// placeholder input of step 5 names no project, so the row reads the
// projects from their records. A finding step 5 itself reports under
// the slug, and a step 5 that runs no such check, are reported first,
// as for every pre row. The fix hint points at J1 step 5 (REQ-0395).
var _ = register(row{slug: slugSigningSocket, run: checkSigningSocket})

func checkSigningSocket(ctx context.Context, h *host) Check {
	if c := h.preCheck(ctx, slugSigningSocket); c.Result != resultPass {
		return c
	}
	r, err := h.loadRecords()
	if err != nil {
		return errorOf(slugSigningSocket, err)
	}
	var users, found []string
	for _, name := range r.names {
		s := recordSigning(h, r.projects[name])
		if s == nil {
			continue
		}
		users = append(users, name)
		if err := preflight.CheckSigning(ctx, name, *s); err != nil {
			found = append(found, err.Error())
		}
	}
	switch {
	case len(found) > 0:
		return fail(slugSigningSocket, joined(found)+"; "+preflight.SigningSocket.Fix)
	case len(users) == 0:
		return pass(slugSigningSocket, "no approved project uses "+preflight.SigningKit)
	}
	return pass(slugSigningSocket, "the signing socket holds the one signing key of "+joined(users))
}

// recordSigning returns what the check reads of the project p: its
// approved product kit git-ssh-sign with its signingKey and the socket
// of the host settings, or nil when its approval holds no such kit.
func recordSigning(h *host, p *state.Project) *preflight.Signing {
	if p == nil || p.Approval == nil {
		return nil
	}
	for _, k := range p.Approval.Set.Kits {
		if k.Source != gate.SourceProduct || k.ID != preflight.SigningKit {
			continue
		}
		s := &preflight.Signing{Key: k.Args[preflight.SigningKeyArg]}
		if h.settings.Signing != nil {
			s.Socket = h.settings.Signing.AgentSocket
		}
		return s
	}
	return nil
}
