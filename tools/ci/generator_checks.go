// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/drift"
)

// The generator of docs/reference/checks.md, the inventory of the checks (12 12.3).
func init() {
	check.RegisterGenerator(check.Generator{Name: "checks page", Outputs: drift.ChecksPageOutput})
}
