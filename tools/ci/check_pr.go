// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/askfirst"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// ask-first is a subcommand and not a check of fast or all: it judges a pull
// request, which a commit alone does not carry (10 10.2, 12 12.4).
func init() {
	check.RegisterCommand(check.Command{
		Name:  "ask-first",
		Usage: []string{"ask-first <payload file>", "ask-first --title <title> --body <file> [--base <rev>]"},
		Run:   askfirst.RunPR,
	})
}
