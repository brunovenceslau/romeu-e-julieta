// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package vet is the vet step of 10 10.2, of the row "format, vet":
// go vet over the whole module, the go command that mise resolves, once
// per target of lint.Targets in the environment of the linter for that
// target, so that a file that builds on darwin alone is vetted by the
// Linux job, which is the one job that runs it.
package vet

import (
	"context"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/lint"
)

// Steps returns the vet steps, one per lint target.
func Steps(_ context.Context, p check.Plan) []check.Step {
	var steps []check.Step
	for _, target := range lint.Targets {
		steps = append(steps, check.Step{
			Name:    "vet " + target.String(),
			Argv:    []string{filepath.Join(p.Tools.GoDir, "go"), "vet", "./..."},
			Environ: lint.Env(target, p.Tools.GoDir),
		})
	}
	return steps
}
