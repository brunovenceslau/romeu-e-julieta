// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import "github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"

var _ = registerBuilder("policy-allow-network", func(fx Fixture) []Call { return []Call{mustCall(PolicyAllowNetwork(fx.Name, fx.Domain))} })

// PolicyAllowNetwork is sbx policy allow network --sandbox <name>
// <domain>: it adds an allow rule scoped to one sandbox (timeout class
// sbx-change), captured. romeu never adds a global rule: --sandbox is
// always there (I22; 07 7.5). Rules "apply to TCP by default"
// (https://docs.docker.com/reference/cli/sbx/policy/allow/network/).
func PolicyAllowNetwork(sandbox, domainName string) (Call, error) {
	return (&argvBuilder{}).lit("policy", "allow", "network", "--sandbox").slot(sandboxName, sandbox).slot(domain, domainName).
		call("policy-allow-network", procgroup.SbxChange, Captured)
}
