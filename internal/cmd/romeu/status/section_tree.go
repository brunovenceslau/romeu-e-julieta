// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package status

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// The reads of the tree section.
const (
	readRender = "render.json"
	readLive   = "live files"
)

// The tree section: what <name>-env/ holds against its live
// render.json, read once and with no lock.
//
// An interrupted promotion (REQ-0386) is only reported: a mutating
// command recovers it, by finishing or abandoning it (REQ-0087). While
// the marker is there, the record's commit and digests are those of a
// render not yet promoted (on a first promotion, the candidate's own), so
// neither the synced commit nor recreate needed is read from it, and no
// drift is either. Otherwise the block gets the config commit the render
// read the spec at, the live files that drifted (REQ-0385; with no
// render.json, a live sbxenv.yaml), and recreate needed (01 1.7): the
// live render's recreate digest differs from the one the open
// generation was created with, for a sandbox sbx lists (REQ-0352,
// REQ-0353), never for an adopted generation, which has none
// (REQ-0233).
var _ = register(section{
	name:  "tree",
	order: 30,
	fill: func(_ context.Context, src *source, p *Project) {
		if src.envDir == "" {
			return
		}
		rec, found, err := render.ReadLive(src.envDir)
		if errors.Is(err, render.ErrRecordInvalid) {
			err = state.RecordUnreadable.Wrap(err, "render.json does not decode", cli.Detail{Key: "file", Value: filepath.Join(src.envDir, render.LiveRecord)})
		}
		if err != nil {
			p.Errors = append(p.Errors, readError(readRender, err))
			return
		}
		if found && rec.Promoting != nil {
			p.Findings = append(p.Findings, Finding{
				Slug:    "interrupted-promotion",
				Message: fmt.Sprintf("the promotion of candidate %s was interrupted; the next mutating command on %s finishes or abandons it", rec.Promoting.CandidateID, src.name),
			})
			return
		}
		var live *render.Record
		if found {
			live = &rec
			p.SyncedCommit = rec.SpecSource.Commit
		}
		drifted, err := render.Drift(src.envDir, live)
		switch {
		case err != nil:
			p.Errors = append(p.Errors, readError(readLive, err))
		case len(drifted) > 0:
			p.Findings = append(p.Findings, Finding{
				Slug: "drift-detected",
				Message: fmt.Sprintf("the live files %s differ from render.json; move the edit into the spec, or run romeu sync --overwrite-drift %s, which loses it",
					strings.Join(drifted, ", "), src.name),
			})
		}
		if live == nil || src.record == nil || len(src.record.Generations) == 0 {
			return
		}
		if _, ok := src.sandbox(); !ok || !src.listed {
			return
		}
		// A generation that is not closed is the last of the record.
		g := src.record.Generations[len(src.record.Generations)-1]
		if g.State == state.GenerationOpen && !g.Adopted && g.CreatedWith != rec.RecreateDigest {
			p.Findings = append(p.Findings, Finding{
				Slug:    "recreate-needed",
				Message: fmt.Sprintf("generation %s was created with another recreate digest than the live render's; run romeu recreate %s", g.ID, src.name),
			})
		}
	},
	text: func(out *cli.Writer, p *Project) error {
		if p.SyncedCommit == "" {
			return nil
		}
		_, err := fmt.Fprintf(out, "  synced commit: %s\n", line(p.SyncedCommit))
		return err
	},
})
