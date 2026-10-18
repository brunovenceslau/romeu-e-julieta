// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/drift"
)

func init() {
	check.Register(check.Check{
		Name:    "generated",
		Guards:  "the generated files are what go generate writes, each tracked as a regular file, and go mod tidy changes neither go.mod nor go.sum",
		Row:     "generated",
		Req:     "REQ-0822",
		Usage:   "go run ./tools/ci generated",
		Fast:    true,
		All:     true,
		Order:   30,
		Package: "tools/ci/internal/drift",
		Commands: []check.Command{
			{Name: "generated", Usage: []string{"generated"}, Run: drift.Run},
			{Name: "generate", Usage: []string{"generate"}, Run: drift.Generate},
		},
		Steps: drift.Steps,
	})
}
