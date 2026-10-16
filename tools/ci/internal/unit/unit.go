// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package unit is the unit step of 10 10.2: the tests of the change,
// the first step that runs code of the change. It names the two
// directories the row names, and leaves out one that does not exist
// yet, which "go test" would take as an error. It passes "-count=1":
// tests that scan the repository would otherwise be served from the
// test cache, which cannot see a file added since the cached run.
package unit

import (
	"context"
	"os"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// Steps returns the unit step for the tree at p.Root.
func Steps(_ context.Context, p check.Plan) []check.Step {
	argv := []string{filepath.Join(p.Tools.GoDir, "go"), "test", "-count=1"}
	for _, dir := range []string{"internal", "tools"} {
		if info, err := os.Stat(filepath.Join(p.Root, dir)); err == nil && info.IsDir() {
			argv = append(argv, "./"+dir+"/...")
		}
	}
	return []check.Step{{Name: "unit", Argv: argv, Environ: check.StepEnv(p.Tools.GoDir)}}
}
