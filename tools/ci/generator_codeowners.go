// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/askfirst"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// The generator of .github/CODEOWNERS (12 12.3), which lives with the
// ask-first list in askfirst.
func init() {
	check.RegisterGenerator(check.Generator{Name: "codeowners", Outputs: askfirst.CodeownersOutput})
}
