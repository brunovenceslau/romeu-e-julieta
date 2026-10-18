// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package crossbuild is the cross-build step of 10 10.2 (REQ-0838):
// romeu builds for darwin amd64 and arm64, and julieta for linux amd64
// and arm64, each with cgo off, the go command that mise resolves and
// the environment of the steps, on every runner.
package crossbuild

import (
	"context"
	"os"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// Build is one binary on one platform.
type Build struct {
	Binary, GOOS, GOARCH string
}

// Builds are the platforms each binary ships for (12 12.1): romeu on
// darwin, julieta on linux, each on amd64 and arm64.
var Builds = []Build{
	{"romeu", "darwin", "amd64"},
	{"romeu", "darwin", "arm64"},
	{"julieta", "linux", "amd64"},
	{"julieta", "linux", "arm64"},
}

// Steps returns one step per build: go build of ./cmd/<binary> for its
// platform, with the binary written to the null device, so the step
// leaves nothing in the tree.
func Steps(_ context.Context, p check.Plan) []check.Step {
	var steps []check.Step
	for _, b := range Builds {
		steps = append(steps, check.Step{
			Name: "cross build " + b.Binary + " " + b.GOOS + "/" + b.GOARCH,
			Argv: []string{filepath.Join(p.Tools.GoDir, "go"), "build", "-o", os.DevNull, "./cmd/" + b.Binary},
			Environ: append(check.StepEnv(p.Tools.GoDir),
				"GOOS="+b.GOOS,
				"GOARCH="+b.GOARCH,
				"CGO_ENABLED=0",
			),
		})
	}
	return steps
}
