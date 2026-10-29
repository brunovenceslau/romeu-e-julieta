// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/askfirst"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// pr is a subcommand and not a check of fast or all: it judges a pull
// request, which a commit alone does not carry (10 10.2, 12 12.4).
func init() {
	check.RegisterCommand(check.Command{
		Name:  "pr",
		Usage: []string{"pr <payload file>", "pr --title <title> --body <file> [--base <rev>]"},
		Run:   askfirst.RunPR,
	})
}
