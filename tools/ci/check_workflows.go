// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/workflows"
)

func init() {
	check.Register(check.Check{
		Name:   "workflows",
		Guards: "every workflow keeps the closed grammar of 10 10.2",
		Row:    "workflows",
		// The workflows row of 10 10.2 holds no requirement id of its own.
		Req:     "REQ-1241",
		Usage:   "go run ./tools/ci workflows",
		Fast:    true,
		All:     true,
		Order:   110,
		Package: "tools/ci/internal/workflows",
		Commands: []check.Command{
			{Name: "workflows", Usage: []string{"workflows"}, Run: workflows.Run},
		},
		Judge: workflows.Judge,
	})
}
