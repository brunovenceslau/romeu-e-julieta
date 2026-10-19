// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/container"
)

func init() {
	check.Register(check.Check{
		Name:     "container",
		Guards:   "the e2e suite passes on the runners the runner-class table selects, with at least one test, and a precondition it cannot meet exits as an infrastructure error",
		Row:      "e2e",
		Req:      "REQ-0837",
		Usage:    "go test -json -race -count=1 -timeout=14m -tags e2e ./e2e/... (Linux)",
		All:      true,
		RunsCode: true,
		Order:    85,
		Package:  "tools/ci/internal/container",
		Steps:    container.Steps,
	})
}
