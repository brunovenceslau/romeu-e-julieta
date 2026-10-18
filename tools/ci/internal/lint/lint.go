// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package lint is the lint check of 10 10.2: golangci-lint over the
// whole module with the committed .golangci.yml, and forbidigo with
// forbidigo.yml over the packages 04 4.1 does not admit its patterns in,
// each once per lint target. Its tests (config_*_test.go) prove, linter
// by linter, that the configurations still report each checker they
// enable, and hold the repository to the rules the linter cannot see.
package lint

import (
	"context"
	"errors"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// Args are the arguments of every golangci-lint run, the one list
// that the lint steps and the tests that run the linter hold:
//
//   - "--config" names .golangci.yml, so that no other .golangci.* file
//     takes its place.
//   - "-j 1" (golangci-lint v2.14.0, pkg/commands/flagsets.go:
//     --concurrency) limits golangci-lint's own Go threads to one
//     (GOMAXPROCS, pkg/commands/run.go); child processes such as "go
//     list" and the compiler are not capped. It does not fix the lock
//     failure below. A cold run takes about 14s against about 7s by
//     default, and warm runs are about the same. It stays to limit CPU
//     use when runs overlap.
//   - "--allow-parallel-runners" (same file) turns off the file lock
//     that a run takes on golangci-lint.lock in the temporary directory
//     (pkg/commands/run.go, acquireFileLock). Without it, a run fails
//     with "parallel golangci-lint is running" after 5 seconds whenever
//     any other run on the machine, of any repository, holds the lock.
//     The lock is no check: it only keeps runs from overlapping.
var Args = []string{"run", "--config", ".golangci.yml", "-j", "1", "--allow-parallel-runners"}

// Argv returns the command line of a golangci-lint run: the path of
// the linter, then Args.
func Argv(linter string) []string {
	return append([]string{linter}, Args...)
}

// Target is one platform the lint step lints for, named as the go
// command names it.
type Target struct {
	GOOS, GOARCH string
}

func (t Target) String() string {
	return t.GOOS + "/" + t.GOARCH
}

// Targets are the supported targets, linux and darwin each on amd64
// and arm64, and so the platforms the lint step runs for. Every one runs
// with cgo off (Env), so a file that builds only with cgo is a file
// no target builds. The tests that check that the linter reaches each
// Go file read this same list.
var Targets = []Target{
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
}

// Env is the environment of the linter for the target: check.StepEnv,
// then the target, the same for every machine, with cgo off.
func Env(target Target, goDir string) []string {
	return append(check.StepEnv(goDir),
		"GOOS="+target.GOOS,
		"GOARCH="+target.GOARCH,
		"CGO_ENABLED=0",
	)
}

// Steps returns the lint steps: for each of Targets, the run of
// .golangci.yml over the whole module and then the run of forbidigo over
// the packages outside Allowed (BoundaryArgs); a list with no package in
// it fails that step.
func Steps(_ context.Context, p check.Plan) []check.Step {
	boundary, err := BoundaryArgs(p.Root)
	var steps []check.Step
	for _, target := range Targets {
		env := Env(target, p.Tools.GoDir)
		steps = append(steps, check.Step{
			Name:    "lint " + target.String(),
			Argv:    Argv(p.Tools.Linter),
			Environ: env,
		})
		switch {
		case err != nil:
			steps = append(steps, check.Step{Name: "lint boundaries " + target.String(), Unavailable: err})
		case boundary == nil:
			// Every package in the allow list, or none with Go files:
			// the step would judge nothing, and a run that judges
			// nothing must not read as a pass.
			steps = append(steps, check.Step{Name: "lint boundaries " + target.String(), Unavailable: errors.New("no package outside the allow list (Allowed) to judge")})
		case boundary != nil:
			steps = append(steps, check.Step{
				Name:    "lint boundaries " + target.String(),
				Argv:    append([]string{p.Tools.Linter}, boundary...),
				Environ: env,
			})
		}
	}
	return steps
}
