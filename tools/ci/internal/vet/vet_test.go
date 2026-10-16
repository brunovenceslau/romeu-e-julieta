// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package vet

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// TestSteps checks the vet step: go vet of the pinned go over the
// module, in the environment of the steps.
func TestSteps(t *testing.T) {
	assert.Equal(t, []check.Step{{
		Name:    "vet",
		Argv:    []string{"/pinned/bin/go", "vet", "./..."},
		Environ: check.StepEnv("/pinned/bin"),
	}}, Steps(t.Context(), check.Plan{Root: "/src", Tools: check.Tools{GoDir: "/pinned/bin"}}))
}
