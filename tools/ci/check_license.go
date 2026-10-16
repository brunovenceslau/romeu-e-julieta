// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/license"
)

func init() {
	check.Register(check.Check{
		Name:    "license",
		Guards:  "every file carries its license and copyright, as REUSE 3.3 asks",
		Row:     "license",
		Req:     "REQ-0847",
		Usage:   "reuse lint, from the image pinned by digest",
		All:     true,
		Order:   60,
		Package: "tools/ci/internal/license",
		Steps:   license.Steps,
	})
}
