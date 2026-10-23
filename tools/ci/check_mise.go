// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
)

func init() {
	check.Register(check.Check{
		Name:     "mise",
		Guards:   "the mise.lock of this repository and of each example holds every tool of its mise.toml on linux-x64 and linux-arm64, by the logic of julieta lock --check, run as the julieta built from this tree",
		Row:      "mise",
		Req:      "REQ-0845",
		Usage:    "go run ./tools/ci mise",
		All:      true,
		Order:    78,
		RunsCode: true,
		Package:  "tools/ci/internal/mise",
		Commands: []check.Command{
			{Name: "mise", Usage: []string{"mise"}, Run: mise.Run},
		},
		Steps: mise.Steps,
	})
}
