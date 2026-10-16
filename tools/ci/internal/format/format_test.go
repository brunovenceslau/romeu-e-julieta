// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package format

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// TestSteps checks the format step: the gofmt of the pinned go, quiet,
// in the environment of the steps.
func TestSteps(t *testing.T) {
	assert.Equal(t, []check.Step{{
		Name:    "format",
		Argv:    []string{"/pinned/bin/gofmt", "-l", "."},
		Quiet:   true,
		Environ: check.StepEnv("/pinned/bin"),
	}}, Steps(t.Context(), check.Plan{Root: "/src", Tools: check.Tools{GoDir: "/pinned/bin"}}))
}
