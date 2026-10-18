// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package drift

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/adrdir"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/askfirst"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// The tests of this package run generate and generated as tools/ci
// does, with the generated check in the registry of the test binary, as
// tools/ci/check_generated.go puts it in the registry of tools/ci. Its
// generators are those of CODEOWNERS and of the index of the decision
// records: the inventory of the checks of a registry that holds this
// fixture alone says nothing, and TestChecksPage and the tests of
// tools/ci cover it.
func init() {
	check.Register(check.Check{
		Name:    "generated",
		Guards:  "a fixture of the generated check",
		Order:   30,
		Package: "tools/ci/internal/drift",
	})
	check.RegisterGenerator(check.Generator{Name: "codeowners", Outputs: askfirst.CodeownersOutput})
	check.RegisterGenerator(check.Generator{Name: "adr index", Outputs: adrdir.IndexOutput})
}
