// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import "github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"

var _ = registerBuilder("policy-check-network", func(fx Fixture) []Call { return []Call{mustCall(PolicyCheckNetwork(fx.Name, fx.Domain))} })

// PolicyCheckNetwork is sbx policy check network --sandbox <name>
// <domain>: it asks whether the policy of one sandbox allows a host; the
// check is read-only, and its timeout class is sbx-change, since the
// table of 04 4.1 puts the check with the policy calls that change
// something. Captured. romeu runs it after it writes its rules (04 4.2,
// run step 4), and a bare host is evaluated with port 443
// (https://docs.docker.com/reference/cli/sbx/policy/check/network/).
func PolicyCheckNetwork(sandbox, domainName string) (Call, error) {
	return (&argvBuilder{}).lit("policy", "check", "network", "--sandbox").slot(sandboxName, sandbox).slot(domain, domainName).
		call("policy-check-network", procgroup.SbxChange, Captured)
}
