// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import "github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"

// The generator of docs/reference/checks.md (12 12.3), which lives with
// the registry it lists in check.
func init() {
	check.RegisterGenerator(check.Generator{Name: "checks page", Outputs: check.ChecksPageOutput})
}
