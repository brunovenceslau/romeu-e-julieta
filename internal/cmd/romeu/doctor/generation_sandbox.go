// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// The check generation-sandbox (04 4.2, REQ-0389): a project with an
// open generation and no sandbox in sbx ls --json, or a sandbox of a
// project's name whose record has no generation in open, salvaging or
// removing. A sandbox whose name is no project's (sbxdrv's Foreign) or
// whose project has no record is no finding: romeu acts on none of
// those (I26), and adopt is how one is taken in.
var _ = register(row{slug: "generation-sandbox", run: checkGenerationSandbox})

func checkGenerationSandbox(ctx context.Context, h *host) Check {
	const slug = "generation-sandbox"
	r, err := h.loadRecords()
	if err != nil {
		return errorOf(slug, err)
	}
	sandboxes, err := h.sandboxes(ctx)
	if err != nil {
		return errorOf(slug, err)
	}
	var found []string
	for _, name := range r.names {
		gens := r.projects[name].Generations
		open := slices.ContainsFunc(gens, inState(state.GenerationOpen))
		live := open || slices.ContainsFunc(gens, inState(state.GenerationSalvaging)) || slices.ContainsFunc(gens, inState(state.GenerationRemoving))
		switch present := listed(sandboxes, name); {
		case open && !present:
			found = append(found, name+": an open generation and no sandbox")
		case present && !live:
			found = append(found, name+": a sandbox and no generation in open, salvaging or removing")
		}
	}
	if len(found) > 0 {
		return fail(slug, joined(found))
	}
	return pass(slug, "")
}

// inState returns the test of a generation in the state st.
func inState(st state.State) func(state.Generation) bool {
	return func(g state.Generation) bool { return g.State == st }
}

// listed reports whether sbx lists the sandbox of the project name.
func listed(sandboxes []sbxdrv.Sandbox, name string) bool {
	return slices.ContainsFunc(sandboxes, func(s sbxdrv.Sandbox) bool { return !s.Foreign && s.Name == name })
}
