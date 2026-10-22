// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"context"
	"fmt"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks/actions"
	memwrite "github.com/brunovenceslau/romeu-e-julieta/internal/memstore/write"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// recordSnapshotsDisabled is the step of setup that handles a repo whose
// own config sets core.hooksPath, which bypasses the dispatcher, so no
// commit takes a snapshot (08 8.4, REQ-0692): it writes disabled: true
// into the repo's snapshot/heads.json (disabled: false back, when the
// key is gone) and warns snapshots-disabled for the repo's dir, which
// julieta status and the next SessionStart show from julieta state.
// A repo setup could not clone is left out, as its own error says; a
// read or write that fails is an entry of the report's errors and the
// other repos go on.
func recordSnapshotsDisabled(ctx context.Context, r *run) {
	for _, repo := range r.m.Repos {
		if i := slices.IndexFunc(r.report.Repos, func(o Repo) bool { return o.Dir == repo.Dir }); i >= 0 && r.report.Repos[i].Error != "" {
			continue
		}
		disabled, err := actions.RecordDisabled(ctx, actions.SnapshotConfig{
			Git: r.git, Dir: repo.Path, Memory: repo.Memory, Lock: memwrite.LockPath(r.home, repo.Dir), Now: r.now, ULIDs: r.ulids,
		})
		if err != nil {
			r.report.Errors = append(r.report.Errors, fmt.Sprintf("snapshots disabled of %s: %s", termsafe.Escape(repo.Dir), termsafe.Escape(err.Error())))
		}
		if disabled {
			r.warn(SnapshotsDisabled, "snapshots disabled for "+termsafe.Escape(repo.Dir))
		}
	}
}
