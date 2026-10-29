// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// usage returns the error usage (RJ-349) for a precondition the command
// stops at, named by the slug of the rule it breaks (REQ-1258: a rule
// with no row of 04 4.4 has no id of its own), with exit 2 and the hint
// of the help of the command.
func usage(slug, message string) error {
	e := cli.Usage.New(slug + ": " + message)
	e.Fill = map[string]string{"<binary>": "romeu", " <command>": " sync"}
	return e
}
