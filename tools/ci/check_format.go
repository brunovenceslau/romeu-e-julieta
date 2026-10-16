// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/format"
)

func init() {
	check.Register(check.Check{
		Name:    "format",
		Guards:  "every Go file is formatted as gofmt formats it",
		Row:     "format, vet",
		Req:     "REQ-0820",
		Usage:   "gofmt -l .",
		Fast:    true,
		All:     true,
		Order:   10,
		Package: "tools/ci/internal/format",
		Steps:   format.Steps,
	})
}
