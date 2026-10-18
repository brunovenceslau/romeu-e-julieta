// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/imports"
)

func init() {
	check.Register(check.Check{
		Name:    "imports",
		Guards:  "the packages keep the module boundaries of 10 10.7, and romeu links no terminal, hooks or memory-writer code",
		Row:     "imports",
		Req:     "REQ-0839",
		Usage:   "go run ./tools/ci imports",
		All:     true,
		Order:   95,
		Package: "tools/ci/internal/imports",
		Commands: []check.Command{
			{Name: "imports", Usage: []string{"imports"}, Run: imports.Run},
		},
		Steps: imports.Steps,
	})
}
