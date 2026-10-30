// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package status

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// source is what the sections of one project's block read: the
// project's name, its record (nil when it has none or it did not read)
// with the record's state, and the listing of sbx (listed false when it
// failed).
type source struct {
	name      string
	record    *state.Project
	recorded  string
	sandboxes []sbxdrv.Sandbox
	listed    bool
}

// sandbox returns the project's sandbox in the listing, the one whose
// name is the project's (01 1.1).
func (s *source) sandbox() (sbxdrv.Sandbox, bool) {
	i := slices.IndexFunc(s.sandboxes, func(b sbxdrv.Sandbox) bool { return !b.Foreign && b.Name == s.name })
	if i < 0 {
		return sbxdrv.Sandbox{}, false
	}
	return s.sandboxes[i], true
}

// section is one section of a project's block: fill sets its fields of
// the block, and text prints them; order places it in the text, lowest
// first.
type section struct {
	name  string
	order int
	fill  func(ctx context.Context, src *source, p *Project)
	text  func(out *cli.Writer, p *Project) error
}

// sections is the registry of the block's sections, an unordered set
// (plan rule 14, the seam of T048): each section registers itself from
// its own file with var _ = register(section{...}), and the tasks that
// add a field add a file: the julieta fields with T066, the signing
// state with T080. It is written during initialization and read-only
// after.
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
