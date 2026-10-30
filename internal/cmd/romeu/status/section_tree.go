// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package status

import (
	"context"
	"errors"
	"fmt"
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

// The tree section: what <name>-env/ holds against its render.json. The
// config commit the live render read the spec at; an interrupted
// promotion (REQ-0386), which only a mutating command recovers
// (REQ-0087); else the live files that drifted (REQ-0385); and recreate
// needed, the live render's recreate digest against the one the live
// generation was created with (01 1.7), never for an adopted one, which
// has none (REQ-0233). Nothing here takes the lock or recovers.
var _ = register(section{
	name:  "tree",
	order: 30,
	fill: func(_ context.Context, src *source, p *Project) {
		if src.envDir == "" {
			return
		}
		rec, found, err := render.ReadLive(src.envDir)
		if errors.Is(err, render.ErrRecordInvalid) {
			err = state.RecordUnreadable.Wrap(err, "render.json does not decode", cli.Detail{Key: "file", Value: src.envDir + "/" + render.LiveRecord})
		}
		if err != nil {
			p.Errors = append(p.Errors, readError(readRender, err))
			return
		}
		if !found {
			return
		}
		p.SyncedCommit = rec.SpecSource.Commit
		if rec.Promoting != nil {
			p.Findings = append(p.Findings, Finding{
				Slug:    "interrupted-promotion",
				Message: fmt.Sprintf("the promotion of candidate %s was interrupted; the next romeu sync %s recovers it", rec.Promoting.CandidateID, src.name),
			})
		} else {
			drifted, err := render.CheckDrift(src.envDir)
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
		}
		if src.record == nil || src.record.Current() == state.None {
			return
		}
		// A generation that is not closed is the last of the record.
		g := src.record.Generations[len(src.record.Generations)-1]
		if !g.Adopted && g.CreatedWith != rec.RecreateDigest {
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
