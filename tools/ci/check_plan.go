// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/plan"
)

func init() {
	check.Register(check.Check{
		Name:   "plan",
		Guards: "the task files of docs/plan/tasks keep the fields, the ids, the edges and the overlap rule of docs/plan.md",
		// The task half of the sequences row of 10 10.2, which plan holds
		// until sequences lands.
		Row:     "sequences",
		Req:     "REQ-0826",
		Usage:   "go run ./tools/ci plan [--critical-path | --ready]",
		Fast:    true,
		All:     true,
		Order:   125,
		Package: "tools/ci/internal/plan",
		Commands: []check.Command{
			{Name: "plan", Usage: []string{"plan [--critical-path | --ready]"}, Run: plan.Run},
		},
		Judge: plan.Judge(plan.Generated),
	})
}
