// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/generated"
)

func init() {
	check.Register(check.Check{
		Name:    "generated",
		Guards:  "the generated files are what go generate writes, each tracked as a regular file",
		Row:     "generated",
		Req:     "REQ-0822",
		Usage:   "go run ./tools/ci generated",
		Fast:    true,
		All:     true,
		Order:   30,
		Package: "tools/ci/internal/generated",
		Commands: []check.Command{
			{Name: "generated", Usage: []string{"generated"}, Run: generated.Run},
			{Name: "generate", Usage: []string{"generate"}, Run: generated.Generate},
		},
		Steps:   generated.Steps,
		Outputs: generated.Outputs,
	})
}
