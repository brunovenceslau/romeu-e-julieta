// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lint

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// TestSteps checks the lint steps: one per target, in the order of
// Targets, each the linter with Args in the environment of its target.
func TestSteps(t *testing.T) {
	steps := Steps(t.Context(), check.Plan{Root: "/src", Tools: check.Tools{Linter: "/pinned/golangci-lint", GoDir: "/pinned/bin"}})
	var want []check.Step
	for _, target := range Targets {
		want = append(want, check.Step{
			Name:    "lint " + target.GOOS + "/" + target.GOARCH,
			Argv:    slices.Concat([]string{"/pinned/golangci-lint"}, Args),
			Environ: slices.Concat(check.StepEnv("/pinned/bin"), []string{"GOOS=" + target.GOOS, "GOARCH=" + target.GOARCH, "CGO_ENABLED=0"}),
		})
	}
	assert.Equal(t, want, steps)
	assert.Len(t, steps, 4)
}
