// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/fix"
)

func init() {
	check.Register(check.Check{
		Name:    "fix",
		Guards:  "go fix would change no Go file of the module, for each lint target",
		Row:     "fix",
		Req:     "REQ-1254",
		Usage:   "go fix -diff -tags e2e,host ./..., once per lint target",
		Fast:    true,
		All:     true,
		Order:   25,
		Package: "tools/ci/internal/fix",
		Steps:   fix.Steps,
	})
}
