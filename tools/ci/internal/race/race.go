// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package race is the race step of 10 10.2, the row "race": every test
// of the module, the one test run of fast and all, with the race
// detector, writing in all the cover profile that the coverage check
// reads after the steps. It runs
// code of the change, which can game its own result: a test can write
// the cover profile itself, or exercise less than it claims; the review
// of the diff covers that.
//
// The race run, which builds every package with the race detector and
// runs every test, has a budget of its own, the class
// procgroup.ToolRace, and not the one of a fast step; the job of ci.yml
// has 60 minutes.
//
// Argv builds the "go test" command line of this step and of the e2e
// step of tools/ci/internal/container, so that the race flags cannot
// drift apart. Both pass "-count=1", and only because a test in each
// reads the tree through a subprocess, which the go command's test
// cache does not record (it records the files and directories a test
// opens and the variables it reads in its own process): in the race
// run, TestOnlyAllowedDirectives of tools/ci/internal/lint, which lists
// the files with "git ls-files"; in the e2e run, the build of julieta
// by "go build" in e2e/internal/container (BuildJulieta), which a cached
// result would not rebuild after a change to cmd/julieta.
package race

import (
	"context"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// Argv returns the command line of "go test" with the race detector:
// the go command of goDir, -race and -count=1 (see the package doc),
// then -tags tags when tags is not empty, then flags, then pkgs.
func Argv(goDir, tags string, flags []string, pkgs ...string) []string {
	argv := []string{filepath.Join(goDir, "go"), "test", "-race", "-count=1"}
	if tags != "" {
		argv = append(argv, "-tags", tags)
	}
	return append(append(argv, flags...), pkgs...)
}

// Steps returns the race step, which writes the cover profile to
// p.Profile. With no profile, as in fast, which has no step after the
// run to read one, it writes none.
func Steps(_ context.Context, p check.Plan) []check.Step {
	var flags []string
	if p.Profile != "" {
		flags = []string{"-coverprofile=" + p.Profile}
	}
	return []check.Step{{
		Name:    "race",
		Argv:    Argv(p.Tools.GoDir, "", flags, "./..."),
		Environ: check.StepEnv(p.Tools.GoDir),
		Class:   procgroup.ToolRace,
	}}
}
