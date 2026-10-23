// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/golden"
)

func init() {
	check.Register(check.Check{
		Name:    golden.Name,
		Guards:  "no CI invocation rewrites a golden, and every golden a change touches is listed for review",
		Row:     "golden governance",
		Req:     "REQ-0835",
		Usage:   "go run ./tools/ci golden",
		All:     true,
		Order:   118,
		Package: "tools/ci/internal/golden",
		Commands: []check.Command{
			{Name: golden.Name, Usage: []string{"golden"}, Run: golden.Run},
		},
		Judge: golden.Judge,
	})
}
