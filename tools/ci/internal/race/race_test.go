// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package race

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

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
