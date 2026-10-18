// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/crossbuild"
)

func init() {
	check.Register(check.Check{
		Name:    "cross build",
		Guards:  "romeu builds for darwin and julieta for linux, each on amd64 and arm64",
		Row:     "cross-build",
		Req:     "REQ-0838",
		Usage:   "GOOS=<os> GOARCH=<arch> CGO_ENABLED=0 go build -o /dev/null ./cmd/<binary>",
		All:     true,
		Order:   90,
		Package: "tools/ci/internal/crossbuild",
		Steps:   crossbuild.Steps,
	})
}
