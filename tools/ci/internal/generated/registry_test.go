// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package generated

import "github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"

// The tests of this package run generate and generated as tools/ci
// does, with the generated check in the registry of the test binary, as
// tools/ci/check_generated.go puts it in the registry of tools/ci. Its
// outputs are those of its sources: the inventory of the checks of a
// registry that holds this fixture alone says nothing, and TestChecksPage
// and the tests of tools/ci cover it.
func init() {
	check.Register(check.Check{
		Name:    "generated",
		Guards:  "a fixture of the generated check",
		Order:   30,
		Package: "tools/ci/internal/generated",
		Outputs: sources,
	})
}
