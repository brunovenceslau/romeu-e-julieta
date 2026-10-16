// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package race is the race step of 10 10.2, the row "unit + golden +
// race": every test of the module with the race detector, writing the
// cover profile that the coverage check reads after the steps. It runs
// code of the change, which can game its own result: a test can write
// the cover profile itself, or exercise less than it claims; the review
// of the diff covers that.
//
// The race run, which builds every package with the race detector and
// runs every test, has a budget of its own, the class
// procgroup.ToolRace, and not the one of a fast step; the job of ci.yml
// has 60 minutes. It passes "-count=1" for the reason the unit step
// does: a test that scans the repository would otherwise be served from
// the test cache.
package race

import (
	"context"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// Steps returns the race step, which writes the cover profile to
// p.Profile.
func Steps(_ context.Context, p check.Plan) []check.Step {
	return []check.Step{{
		Name:    "race",
		Argv:    []string{filepath.Join(p.Tools.GoDir, "go"), "test", "-race", "-count=1", "-coverprofile=" + p.Profile, "./..."},
		Environ: check.StepEnv(p.Tools.GoDir),
		Class:   procgroup.ToolRace,
	}}
}
