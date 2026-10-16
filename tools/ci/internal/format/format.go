// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package format is the format step of 10 10.2, of the row "format,
// vet": gofmt -l over the tree, the gofmt of the directory of the go
// command that mise resolves, in the environment of the steps. gofmt
// lists the files it would change, so the step passes when it prints
// nothing, and what it prints is paths of the repository (Quiet).
package format

import (
	"context"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// Steps returns the format step.
func Steps(_ context.Context, p check.Plan) []check.Step {
	return []check.Step{{
		Name:    "format",
		Argv:    []string{filepath.Join(p.Tools.GoDir, "gofmt"), "-l", "."},
		Quiet:   true,
		Environ: check.StepEnv(p.Tools.GoDir),
	}}
}
