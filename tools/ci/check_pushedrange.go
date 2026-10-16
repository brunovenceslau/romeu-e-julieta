// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/pushedrange"
)

func init() {
	check.Register(check.Check{
		Name:    "pushed range",
		Guards:  "a push publishes no forbidden name, and no commit of it adds a never-tracked file",
		Row:     "pushed range",
		Req:     "REQ-0825",
		Usage:   "go run ./tools/ci fast <remote> <url>, as the pre-push hook runs it",
		Hook:    true,
		Order:   130,
		Package: "tools/ci/internal/pushedrange",
		Judge:   pushedrange.Judge,
	})
}
