// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import (
	"context"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/signing"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// The signing rule (06 6.4, I34) is keyed by the name of the product
// kit, and the key it checks is that kit's arg.
const (
	// SigningKit is the product kit whose sandboxes reach the dedicated
	// signing socket.
	SigningKit = "git-ssh-sign"
	// SigningKeyArg is the arg of SigningKit that holds the public key
	// commits are signed with.
	SigningKeyArg = "signingKey"
)

// Signing is what the check signing-socket reads of a project that uses
// SigningKit: signing.agentSocket of the host settings, empty when it is
// unset, and the kit's signingKey arg, empty when the spec sets none.
type Signing struct {
	Socket string
	Key    string
}

// SigningOf returns the Signing of a project whose kits are kits, under
// the host settings h, or nil when no kit of the project is the product
// kit SigningKit.
func SigningOf(h *spec.HostSettings, kits []spec.Kit) *Signing {
	for _, k := range kits {
		if k.Product != SigningKit {
			continue
		}
		s := &Signing{Key: k.Args[SigningKeyArg]}
		if h != nil && h.Signing != nil {
			s.Socket = h.Signing.AgentSocket
		}
		return s
	}
	return nil
}

// CheckSigning is the check signing-socket, which sync, step 5 and
// doctor share (REQ-0384, REQ-0600, REQ-0601): nil when the socket of s
// holds exactly one key and it is s.Key, and otherwise the error
// signing-socket (RJ-204) of the project, wrapping the reason of
// internal/signing (no socket, not a socket of this user, unreachable,
// a number of keys other than one, another key, or a signing key that
// is not a public key line).
//
//romeu:invariant I34 guard
func CheckSigning(ctx context.Context, project string, s Signing) error {
	err := signing.Check(ctx, s.Socket, s.Key)
	if err == nil {
		return nil
	}
	return SigningSocket.Wrap(err, "the project "+termsafe.Escape(project)+" uses the kit "+SigningKit+
		", and signing.agentSocket does not hold its signingKey as its one key",
		cli.Detail{Key: "socket", Value: termsafe.Escape(s.Socket)})
}

// checkSigning is the row signing-socket of step 5: the check of the
// project the command is for, when it uses SigningKit.
func checkSigning(ctx context.Context, h *hostEnv) error {
	if h.in.Signing == nil {
		return nil
	}
	return CheckSigning(ctx, h.in.Project, *h.in.Signing)
}
