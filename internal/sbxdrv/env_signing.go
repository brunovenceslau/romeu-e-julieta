// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

// The contribution signing (04 4.1, Subprocesses, item 1; Q19; I34,
// REQ-0520): the sbx calls of a project that uses the product kit
// git-ssh-sign carry SSH_AUTH_SOCK=<signing.agentSocket>, the dedicated
// signing socket, and the calls of every other project, and of no
// project, carry no SSH_AUTH_SOCK, so the host's own agent is never
// forwarded. That the socket holds the one signing key is checked before
// those calls by the rule signing-socket (sync, preflight step 5).
var _ = register("signing", signingEnv)

// signingKit is the product kit the contribution is keyed by, the
// preflight's SigningKit, which this package cannot import.
const signingKit = "git-ssh-sign"

// signingEnv returns SSH_AUTH_SOCK for a call of a project that uses
// signingKit when the host settings set the socket, and nothing
// otherwise.
func signingEnv(in EnvInput) []string {
	if in.Project == nil || in.Host == nil || in.Host.Signing == nil || in.Host.Signing.AgentSocket == "" {
		return nil
	}
	for _, k := range in.Project.Kits {
		if k.Product == signingKit {
			return []string{"SSH_AUTH_SOCK=" + in.Host.Signing.AgentSocket}
		}
	}
	return nil
}
