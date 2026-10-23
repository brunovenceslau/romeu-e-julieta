// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import "github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"

var _ = registerBuilder("policy-rm-network", func(fx Fixture) []Call { return []Call{mustCall(PolicyRmNetwork(fx.Name, fx.Domain))} })

// PolicyRmNetwork is sbx policy rm network --sandbox <name> --resource
// <domain>: it removes a rule romeu applied, by its resource, from the
// policy of one sandbox (timeout class sbx-change), captured. The shape
// is the one of 10 10.3 (Known argv). Whether sbx asks to confirm the
// removal, which a call with its stdin closed could not answer, is not
// documented for a removal by resource; the call does not carry
// --force, and probe A9 records the answer.
// (https://docs.docker.com/reference/cli/sbx/policy/rm/network/).
func PolicyRmNetwork(sandbox, domainName string) (Call, error) {
	return (&argvBuilder{}).lit("policy", "rm", "network", "--sandbox").slot(sandboxName, sandbox).lit("--resource").slot(domain, domainName).
		call("policy-rm-network", procgroup.SbxChange, Captured)
}
