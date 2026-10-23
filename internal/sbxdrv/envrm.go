// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import "github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"

var _ = registerBuilder("env-rm", func(fx Fixture) []Call { return []Call{mustCall(EnvRm(fx.Dir))} })

// EnvRm is sbx env rm --force <dir>: it removes the sandbox and the
// secrets scoped to it (timeout class sbx-change), captured. This is the
// one call that carries --force, which on env rm "skip[s] confirmation
// prompts and delete[s] even if in use": a call with its stdin closed
// cannot answer the prompt, and sbx v0.48.0 fails a confirmation without
// a terminal with exit 1
// (https://docs.docker.com/reference/cli/sbx/env/rm/;
// https://docs.docker.com/ai/sandboxes/release-notes/, 0.48.0, "Update
// non-interactive workflows"). The confirmation that matters is romeu's
// own, which comes before the salvage and before this call (04 4.2, rm);
// --force on env rm does not approve a plan (guard.go, I4), and it does
// not carry --prune-bindings, so the global bindings stay.
func EnvRm(dir string) (Call, error) {
	return (&argvBuilder{}).lit("env", "rm", "--force").slot(envDir, dir).call("env-rm", procgroup.SbxChange, Captured)
}
