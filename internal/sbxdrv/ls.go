// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import "github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"

var _ = registerBuilder("ls", func(Fixture) []Call { return []Call{List()} })

// List is sbx ls --json: a read (timeout class sbx-read), captured. Its
// output is the list of sandboxes; the identity step of the preflight, the
// generation machine and run step 3 read it.
func List() Call {
	return mustCall((&argvBuilder{}).lit("ls", "--json").call("ls", procgroup.SbxRead, Captured))
}
