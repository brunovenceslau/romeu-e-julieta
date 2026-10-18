// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"os"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/schema"
	"github.com/brunovenceslau/romeu-e-julieta/tools/schemagen"
)

// The generator of schemas/*.json (12 12.3): one schema for each format
// registered with tools/schemagen.
func init() {
	check.RegisterGenerator(check.Generator{Name: "schemas", Outputs: func(*os.Root) ([]check.Output, error) {
		return schema.Outputs(schemagen.Registered())
	}})
}
