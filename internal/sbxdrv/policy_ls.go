// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import "github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"

var _ = registerBuilder("policy-ls", func(Fixture) []Call { return []Call{PolicyList()} })

// PolicyList is sbx policy ls --json: the active policies, global ones
// included, which the checks of I22 read (a broad allow rule), a read
// (timeout class sbx-read), captured. "Use --json for the filtered
// daemon response"
// (https://docs.docker.com/reference/cli/sbx/policy/ls/). No parser reads
// its output yet: the shape of a policy row is not documented, and the
// preflight that reads it records the shape it parses (T046).
func PolicyList() Call {
	return mustCall((&argvBuilder{}).lit("policy", "ls", "--json").call("policy-ls", procgroup.SbxRead, Captured))
}
