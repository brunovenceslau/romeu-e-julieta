// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/taggedsuites"
)

func init() {
	check.Register(check.Check{
		Name:    "tagged suites",
		Guards:  "the e2e and host suites build with their tags on every runner",
		Row:     "build the tagged suites",
		Req:     "REQ-0832",
		Usage:   "go vet -tags e2e ./e2e/... and go vet -tags host ./e2e/host/...",
		All:     true,
		Order:   75,
		Package: "tools/ci/internal/taggedsuites",
		Steps:   taggedsuites.Steps,
	})
}
