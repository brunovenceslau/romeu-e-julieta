// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import "github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"

var _ = registerBuilder("env-plan", func(fx Fixture) []Call { return []Call{mustCall(EnvPlan(fx.Dir))} })

// EnvPlan is sbx env plan <dir>: a read (timeout class sbx-read),
// captured. It shows what applying the environment would change and
// changes, approves and records nothing
// (https://docs.docker.com/reference/cli/sbx/env/plan/); its output is
// text, which romeu does not read.
func EnvPlan(dir string) (Call, error) {
	return (&argvBuilder{}).lit("env", "plan").slot(envDir, dir).call("env-plan", procgroup.SbxRead, Captured)
}
