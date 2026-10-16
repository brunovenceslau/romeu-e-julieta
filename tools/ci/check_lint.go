// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/lint"
)

func init() {
	check.Register(check.Check{
		Name:    "lint",
		Guards:  "the linters of .golangci.yml find nothing, for each lint target",
		Row:     "lint",
		Req:     "REQ-0823",
		Usage:   "golangci-lint run --config .golangci.yml, once per lint target",
		Fast:    true,
		All:     true,
		Order:   40,
		Package: "tools/ci/internal/lint",
		Steps:   lint.Steps,
	})
}
