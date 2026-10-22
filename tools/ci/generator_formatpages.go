// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/schema"
)

// The generator of the reference page of each file format (S10, 12
// 12.3): docs/reference/<name>.md for each page registered with the
// schema package, beside the format's schema.
func init() {
	check.RegisterGenerator(check.Generator{Name: "format reference", Outputs: schema.PageOutputs})
}
