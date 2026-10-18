// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/drift"
)

// The generator of .github/CODEOWNERS, from .github/ask-first.yaml (12 12.3).
func init() {
	check.RegisterGenerator(check.Generator{Name: "codeowners", Outputs: drift.Codeowners})
}
