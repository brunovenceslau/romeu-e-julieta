// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/schema"
)

func init() {
	check.Register(check.Check{
		Name:    "schema",
		Guards:  "the committed schemas are the generated ones, the examples and the testdata files of a format validate, and each example names the next release, or, a kit descriptor, carries the modeline of the kit schema of sbx and validates against it",
		Row:     "schema",
		Req:     "REQ-0836",
		Usage:   "go run ./tools/ci schema",
		All:     true,
		Order:   115,
		Package: "tools/ci/internal/schema",
		Commands: []check.Command{
			{Name: "schema", Usage: []string{"schema"}, Run: schema.Run},
		},
		Judge: schema.Judge,
	})
}
