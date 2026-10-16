// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/race"
)

func init() {
	check.Register(check.Check{
		Name:     "race",
		Guards:   "every test of the module passes with the race detector, which writes the cover profile",
		Row:      "unit + golden + race",
		Req:      "REQ-0833",
		Usage:    "go test -race -count=1 -coverprofile=<temporary directory>/cover.out ./...",
		All:      true,
		RunsCode: true,
		Order:    80,
		Package:  "tools/ci/internal/race",
		Steps:    race.Steps,
	})
}
