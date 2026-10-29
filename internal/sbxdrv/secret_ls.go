// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import "github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"

var _ = registerBuilder("secret-ls", func(Fixture) []Call { return []Call{SecretList()} })

// SecretList is sbx secret ls: the secrets sbx holds, the global ones
// included, which the check sbx-global-secret of I22 reads. It is a read
// (timeout class sbx-read), captured, and it takes no flag: it prints
// the names of the services, never a value (I25), and romeu does not ask
// it for more.
//
// The argv is the one of the sbx reference
// (https://docs.docker.com/reference/cli/sbx/secret/ls/). Probe A9 has
// not recorded the listing on a host yet (T168, sitting O3): until it
// does, the fake answers with a synthetic session, and the output is read
// by the parser of internal/preflight (listings.go), which refuses any
// shape it does not know as upstream-shape (RJ-310). The parser is there
// and not here because the preflight owns the rule of 03 that decodes the
// bytes of a listing (REQ-0170).
func SecretList() Call {
	return mustCall((&argvBuilder{}).lit("secret", "ls").call("secret-ls", procgroup.SbxRead, Captured))
}
