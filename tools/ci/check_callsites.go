// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/callsites"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

func init() {
	check.Register(check.Check{
		Name:    "callsites",
		Guards:  "romeu deletes nothing outside the admitted-call-site table (I16), and every subprocess starts through internal/procgroup",
		Row:     "callsites",
		Usage:   "go run ./tools/ci callsites",
		All:     true,
		Order:   96,
		Package: "tools/ci/internal/callsites",
		Commands: []check.Command{
			{Name: "callsites", Usage: []string{"callsites"}, Run: callsites.Run},
		},
		Steps: callsites.Steps,
	})
}
