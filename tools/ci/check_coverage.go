// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/coverage"
)

func init() {
	check.Register(check.Check{
		Name:    "coverage",
		Guards:  "every measured package meets its coverage threshold of S9",
		Row:     "coverage",
		Req:     "REQ-0834",
		Usage:   "go run ./tools/ci coverage <profile>",
		All:     true,
		Order:   140,
		Package: "tools/ci/internal/coverage",
		Commands: []check.Command{
			{Name: "coverage", Usage: []string{"coverage <profile>"}, Run: coverage.Run},
		},
		After: coverage.After,
	})
}
