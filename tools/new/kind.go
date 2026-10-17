// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"
	"slices"
	"strings"
)

// kind is one thing tools/new scaffolds. Each kind registers itself
// from the init of its own file, <name>.go, so adding a kind adds a file
// and edits none (rule 4 of docs/plan.md).
type kind struct {
	name string                           // the first argument that selects it
	args string                           // the arguments it takes, as the usage text shows them
	run  func(e env, args []string) error // writes the file; args are the ones after name
}

// registry holds the kinds by name.
type registry map[string]kind

// kinds is the registry of this command, filled by the init functions
// of the kind files.
var kinds = registry{}

// register adds k to kinds.
func register(k kind) { kinds.add(k) }

// add adds k, and panics when k has no name or no run, or when a kind of
// that name is already there, so two files cannot claim one kind.
func (r registry) add(k kind) {
	if k.name == "" || k.run == nil {
		panic(fmt.Sprintf("new: the kind %q has no name or no run", k.name))
	}
	if _, dup := r[k.name]; dup {
		panic(fmt.Sprintf("new: the kind %q is registered twice", k.name))
	}
	r[k.name] = k
}

// sorted returns the kinds in name order.
func (r registry) sorted() []kind {
	var all []kind
	for _, k := range r {
		all = append(all, k)
	}
	slices.SortFunc(all, func(a, b kind) int { return strings.Compare(a.name, b.name) })
	return all
}

// usage returns the usage text: one line per kind, in name order.
func usage() string {
	var b strings.Builder
	b.WriteString("usage:\n")
	for _, k := range kinds.sorted() {
		fmt.Fprintf(&b, "  go run ./tools/new %s %s\n", k.name, k.args)
	}
	return b.String()
}
