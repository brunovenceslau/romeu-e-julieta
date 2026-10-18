// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/adrdir"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// The generator of the index of the decision records (12 12.3), which
// lives with the records' directory rules in adrdir.
func init() {
	check.RegisterGenerator(check.Generator{Name: "adr index", Outputs: adrdir.IndexOutput})
}
