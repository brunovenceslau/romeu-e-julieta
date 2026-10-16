// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package vet is the vet step of 10 10.2, of the row "format, vet":
// go vet over the whole module, the go command that mise resolves, in
// the environment of the steps.
package vet

import (
	"context"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// Steps returns the vet step.
func Steps(_ context.Context, p check.Plan) []check.Step {
	return []check.Step{{
		Name:    "vet",
		Argv:    []string{filepath.Join(p.Tools.GoDir, "go"), "vet", "./..."},
		Environ: check.StepEnv(p.Tools.GoDir),
	}}
}
