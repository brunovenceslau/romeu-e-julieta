// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/catalog"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

func init() {
	check.Register(check.Check{
		Name:    "catalog",
		Guards:  "every domain the egress catalog uses has an explicit upload entry, every tool key is a backend:tool, and no two files declare one domain differently",
		Row:     "catalog",
		Req:     "REQ-0842",
		Usage:   "go run ./tools/ci catalog",
		All:     true,
		Order:   117,
		Package: "tools/ci/internal/catalog",
		Commands: []check.Command{
			{Name: "catalog", Usage: []string{"catalog"}, Run: catalog.Run},
		},
		Judge: catalog.Judge,
	})
}
