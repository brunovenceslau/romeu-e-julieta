// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package drift

import (
	"os"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/clidoc"
	// The assembled command sets, as tools/ci/generator_cli.go links them,
	// so the pages of the commands are those generate writes.
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/all"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/adrdir"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/askfirst"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// The tests of this package run generate and generated as tools/ci
// does, with the generated check in the registry of the test binary, as
// tools/ci/check_generated.go puts it in the registry of tools/ci. Its
// generators are those of CODEOWNERS, of the index of the decision
// records and of the command line (tools/ci/generator_cli.go), so a test
// that reads this repository sees every file go generate writes: the
// inventory of the checks of a registry that holds this fixture alone
// says nothing, and TestChecksPage and the tests of tools/ci cover it.
func init() {
	check.Register(check.Check{
		Name:    "generated",
		Guards:  "a fixture of the generated check",
		Order:   30,
		Package: "tools/ci/internal/drift",
	})
	check.RegisterGenerator(check.Generator{Name: "codeowners", Outputs: askfirst.CodeownersOutput})
	check.RegisterGenerator(check.Generator{Name: "adr index", Outputs: adrdir.IndexOutput})
	check.RegisterGenerator(check.Generator{Name: "error table", Outputs: pages(clidoc.ErrorPages)})
	check.RegisterGenerator(check.Generator{Name: "command reference", Outputs: pages(clidoc.CommandPages)})
}

// pages returns gen as the Outputs of a generator, as
// tools/ci/generator_cli.go does.
func pages(gen func(root *os.Root) ([]clidoc.Page, error)) func(root *os.Root) ([]check.Output, error) {
	return func(root *os.Root) ([]check.Output, error) {
		got, err := gen(root)
		var outs []check.Output
		for _, p := range got {
			outs = append(outs, check.Output{Path: p.Path, Content: p.Content})
		}
		return outs, err
	}
}
