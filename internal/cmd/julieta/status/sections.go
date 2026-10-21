// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package status

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
)

// source is what the sections read: the manifest, $HOME, the git of
// julieta and the state setup recorded.
type source struct {
	home  string
	m     manifest.Manifest
	git   *gitsafe.Runner
	state setup.State
}

// section is one section of the report of julieta status: fill sets
// its fields of the report, and text prints them, the report to out
// and the warnings to errOut, the CLI writer of stderr, where every
// warning goes (04 4.1, Output and Warnings; cli.Warn); order places
// it in the text, lowest first.
type section struct {
	name  string
	order int
	fill  func(ctx context.Context, src *source, rep *Report)
	text  func(out, errOut *cli.Writer, rep *Report) error
}

// sections is the registry of the report sections, an unordered set
// (plan rule 14): each section registers itself from its own file with
// var _ = register(section{...}), and the tasks of the other sections
// of 04 4.3 add a file each: tools installed with T053, the last
// snapshot with T054, the agent memory dir with T035, the agent's hooks
// and skills with T079, and the recorded hook failures with T154. It is
// written during initialization and
// read-only after.
var sections []section

// register adds s to the sections and returns it. A name registered
// already is a programming error.
func register(s section) section {
	if slices.ContainsFunc(sections, func(o section) bool { return o.name == s.name }) {
		panic(fmt.Sprintf("status: the section %s is registered twice", s.name))
	}
	sections = append(sections, s)
	slices.SortStableFunc(sections, func(a, b section) int {
		return cmp.Or(cmp.Compare(a.order, b.order), cmp.Compare(a.name, b.name))
	})
	return s
}
