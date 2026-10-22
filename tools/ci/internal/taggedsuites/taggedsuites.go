// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package taggedsuites is the step "build the tagged suites" of 10 10.2
// (REQ-0832): go vet of each tagged suite with its tag, on every
// runner, so that a suite that does not build fails here and not when
// it first runs. A suite whose directory does not exist yet is left
// out, which go vet would take as an error.
//
// Suites is also the one source of the tags that the lint, fix,
// imports and license steps pass (REQ-1254), through Tags. The host
// suite of 10 10.1 enters Suites in the pull request that writes its
// first file (T075): a tag no file carries is refused by
// TestSuitesTagsAreCarried.
package taggedsuites

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// Suite is one tagged suite: its build tag and the directory of its
// packages, from the root.
type Suite struct {
	Tag, Dir string
}

// Suites are the tagged suites in use: those with at least one file in
// the module. The host suite of 10 10.1 is not here until T075.
var Suites = []Suite{
	{"e2e", "e2e"},
}

// Tags returns the tags of Suites, comma separated, as the go command's
// -tags flag and golangci-lint's --build-tags take them.
func Tags() string {
	tags := make([]string, len(Suites))
	for i, s := range Suites {
		tags[i] = s.Tag
	}
	return strings.Join(tags, ",")
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
