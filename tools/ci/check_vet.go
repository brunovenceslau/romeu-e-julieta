// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/vet"
)

func init() {
	check.Register(check.Check{
		Name:    "vet",
		Guards:  "go vet finds nothing in the module",
		Row:     "format, vet",
		Req:     "REQ-0820",
		Usage:   "go vet ./...",
		Fast:    true,
		All:     true,
		Order:   20,
		Package: "tools/ci/internal/vet",
		Steps:   vet.Steps,
	})
}
