// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package taggedsuites is the step "build the tagged suites" of 10 10.2
// (REQ-0832): go vet of the e2e suite with the tag e2e and of the host
// suite with the tag host, on every runner, so that a suite that does
// not build fails here and not when it first runs (the host suite at
// block B). A suite whose directory does not exist yet is left out,
// which go vet would take as an error.
package taggedsuites

import (
	"context"
	"os"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// Suite is one tagged suite: its build tag and the directory of its
// packages, from the root.
type Suite struct {
	Tag, Dir string
}

// Suites are the two tagged suites of 10 10.1.
var Suites = []Suite{
	{"e2e", "e2e"},
	{"host", "e2e/host"},
}

// Steps returns one go vet step per suite whose directory exists below
// p.Root.
func Steps(_ context.Context, p check.Plan) []check.Step {
	var steps []check.Step
	for _, s := range Suites {
		if info, err := os.Stat(filepath.Join(p.Root, filepath.FromSlash(s.Dir))); err != nil || !info.IsDir() {
			continue
		}
		steps = append(steps, check.Step{
			Name:    "tagged suites " + s.Tag,
			Argv:    []string{filepath.Join(p.Tools.GoDir, "go"), "vet", "-tags", s.Tag, "./" + s.Dir + "/..."},
			Environ: check.StepEnv(p.Tools.GoDir),
		})
	}
	return steps
}
