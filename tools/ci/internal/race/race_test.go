// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package race

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// TestArgv pins the command line both race-detector steps are built
// from: with and without tags, the flags before the packages.
func TestArgv(t *testing.T) {
	assert.Equal(t, []string{"/g/go", "test", "-race", "-count=1", "./..."}, Argv("/g", "", nil, "./..."))
	assert.Equal(t, []string{"/g/go", "test", "-race", "-count=1", "-tags", "e2e", "-json", "-x", "./a/...", "./b"}, Argv("/g", "e2e", []string{"-json", "-x"}, "./a/...", "./b"))
}

// TestSteps checks the race step: every test with the race detector,
// the cover profile at the path of the plan, under the class of the
// race run.
func TestSteps(t *testing.T) {
	assert.Equal(t, []check.Step{{
		Name:    "race",
		Argv:    []string{"/pinned/bin/go", "test", "-race", "-count=1", "-coverprofile=/tmp/p/cover.out", "./..."},
		Environ: check.StepEnv("/pinned/bin"),
		Class:   procgroup.ToolRace,
	}}, Steps(t.Context(), check.Plan{Root: "/src", Tools: check.Tools{GoDir: "/pinned/bin"}, Profile: "/tmp/p/cover.out"}))
}
