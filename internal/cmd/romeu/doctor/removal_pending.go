// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// The check removal-pending (04 4.2, REQ-0390; 01 1.6): a project whose
// generation is in the state removing while its sandbox is still listed
// by sbx ls --json. The hint is romeu rm <name>, which finishes the
// removal an earlier command began.
var _ = register(row{slug: "removal-pending", run: checkRemovalPending})

func checkRemovalPending(ctx context.Context, h *host) Check {
	const slug = "removal-pending"
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
		removing := slices.ContainsFunc(r.projects[name].Generations, func(g state.Generation) bool { return g.State == state.GenerationRemoving })
		if removing && listed(sandboxes, name) {
			found = append(found, name+": a removing generation with its sandbox present; run romeu rm "+name)
		}
	}
	if len(found) > 0 {
		return fail(slug, joined(found))
	}
	return pass(slug, "")
}
