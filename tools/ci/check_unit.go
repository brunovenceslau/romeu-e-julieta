// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/unit"
)

func init() {
	check.Register(check.Check{
		Name:     "unit",
		Guards:   "the tests of internal/ and tools/ pass",
		Row:      "unit",
		Req:      "REQ-0824",
		Usage:    "go test -count=1 ./internal/... ./tools/...",
		Fast:     true,
		All:      true,
		RunsCode: true,
		Order:    70,
		Package:  "tools/ci/internal/unit",
		Steps:    unit.Steps,
	})
}
