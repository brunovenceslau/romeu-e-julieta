// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import "github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"

var _ = registerBuilder("stop", func(fx Fixture) []Call { return []Call{mustCall(Stop(fx.Name))} })

// Stop is sbx stop <name>: it stops a sandbox without removing it
// (timeout class sbx-change), captured. "Stopped sandboxes retain their
// state" (https://docs.docker.com/reference/cli/sbx/stop/).
func Stop(name string) (Call, error) {
	return (&argvBuilder{}).lit("stop").slot(sandboxName, name).call("stop", procgroup.SbxChange, Captured)
}
