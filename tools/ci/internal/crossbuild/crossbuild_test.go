// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package crossbuild

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// TestSteps checks the four builds of REQ-0838: romeu for darwin and
// julieta for linux, each on amd64 and arm64, with cgo off, the pinned
// go and the environment of the steps.
func TestSteps(t *testing.T) {
	step := func(binary, goos, goarch string) check.Step {
		return check.Step{
			Name:    "cross build " + binary + " " + goos + "/" + goarch,
			Argv:    []string{"/pinned/bin/go", "build", "-o", os.DevNull, "./cmd/" + binary},
			Environ: append(check.StepEnv("/pinned/bin"), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0"),
		}
	}
	assert.Equal(t, []check.Step{
		step("romeu", "darwin", "amd64"),
		step("romeu", "darwin", "arm64"),
		step("julieta", "linux", "amd64"),
		step("julieta", "linux", "arm64"),
	}, Steps(t.Context(), check.Plan{Root: "/src", Tools: check.Tools{GoDir: "/pinned/bin"}}))
}
