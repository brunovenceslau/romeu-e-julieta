// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/hygiene"
)

func init() {
	check.Register(check.Check{
		Name:   "hygiene",
		Guards: "the tracked paths and their content keep the hygiene rules and hold no forbidden name",
		Row:    "hygiene",
		// The hygiene row of 10 10.2 holds no requirement id of its own.
		Req:     "",
		Usage:   "go run ./tools/ci hygiene [--file <path>]; go run ./tools/ci hygiene add",
		Fast:    true,
		All:     true,
		Order:   100,
		Package: "tools/ci/internal/hygiene",
		Commands: []check.Command{
			{Name: "hygiene", Usage: []string{"hygiene [--file <path>]", "hygiene add"}, Run: hygiene.Run},
		},
		Judge: hygiene.Judge,
	})
}
