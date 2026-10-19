// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/kits"
)

func init() {
	check.Register(check.Check{
		Name:    "kits",
		Guards:  "every kit keeps the one frontend pin and the install-step rules, and the pin file agrees with ci.yml and with the signed mise checksum file",
		Row:     "kits",
		Req:     "REQ-0843",
		Usage:   "go run ./tools/ci kits",
		Fast:    true,
		All:     true,
		Order:   112,
		Package: "tools/ci/internal/kits",
		Commands: []check.Command{
			{Name: "kits", Usage: []string{"kits"}, Run: kits.Run},
		},
		Judge: kits.Judge,
	})
}
