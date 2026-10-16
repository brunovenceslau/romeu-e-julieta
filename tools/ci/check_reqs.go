// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/reqs"
)

func init() {
	check.Register(check.Check{
		Name:    "reqs",
		Guards:  "the requirement ids of the specification and of the task files keep the rules of 12 12.11",
		Row:     "reqs",
		Req:     "REQ-0829",
		Usage:   "go run ./tools/ci reqs [allocate]",
		Fast:    true,
		All:     true,
		Order:   120,
		Package: "tools/ci/internal/reqs",
		Commands: []check.Command{
			{Name: "reqs", Usage: []string{"reqs [allocate]"}, Run: reqs.Run},
		},
		Judge: reqs.Judge,
	})
}
