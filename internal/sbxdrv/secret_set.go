// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import "github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"

var _ = registerBuilder("secret-set", func(fx Fixture) []Call {
	return []Call{
		mustCall(SecretSet("github", fx.Name, fx.Helper, "")),
		mustCall(SecretSet("github", fx.Name, fx.Helper, "55m")),
	}
})

// SecretSet is sbx secret set <service> --sandbox <name> --command
// <command> [--refresh <duration>]: it stores a command as the source of
// a secret of one sandbox (timeout class sbx-change), captured. sbx runs
// the command on the host when it needs the value; romeu never handles
// a value (I25), so this is the only form of the call, and the guard
// refuses every other (guard.go). The secret is scoped to the sandbox
// and never global (I22). refresh is empty for sbx's default (55m), or
// "on-demand", or a duration
// (https://docs.docker.com/reference/cli/sbx/secret/set/).
func SecretSet(serviceName, sandbox, command, refreshPolicy string) (Call, error) {
	b := (&argvBuilder{}).lit("secret", "set").slot(service, serviceName).lit("--sandbox").slot(sandboxName, sandbox).lit("--command").slot(hostCommand, command)
	if refreshPolicy != "" {
		b.lit("--refresh").slot(refresh, refreshPolicy)
	}
	return b.call("secret-set", procgroup.SbxChange, Captured)
}
