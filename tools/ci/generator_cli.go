// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"os"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/clidoc"
	// The assembled command sets of both binaries, so the reference pages
	// hold every registered command (10 10.7, REQ-0971).
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/all"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// The generators of the command line (12 12.3), which live with their
// data: the pages of the error table in internal/cli and the reference
// pages of the commands in internal/cli/clidoc, which hands tools/ci
// both (ADR 0015, decision 7).
func init() {
	check.RegisterGenerator(check.Generator{Name: "error table", Outputs: pages(clidoc.ErrorPages)})
	check.RegisterGenerator(check.Generator{Name: "command reference", Outputs: pages(clidoc.CommandPages)})
}

// pages returns gen as the Outputs of a generator.
func pages(gen func(root *os.Root) ([]clidoc.Page, error)) func(root *os.Root) ([]check.Output, error) {
	return func(root *os.Root) ([]check.Output, error) {
		got, err := gen(root)
		if err != nil {
			return nil, err
		}
		outs := make([]check.Output, 0, len(got))
		for _, p := range got {
			outs = append(outs, check.Output{Path: p.Path, Content: p.Content})
		}
		return outs, nil
	}
}
