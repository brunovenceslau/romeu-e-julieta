// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// checkDrift is step 7 of 04 4.2 (REQ-0342): each live file and kit dir
// of every target is compared with the digest its current render.json
// holds (render.CheckDrift; I15), before step 8 promotes any target. A
// live file that differs, or is missing, stops the command with
// drift-detected (RJ-303, exit 4) naming the first such file of the
// first target, and nothing is promoted. With --overwrite-drift the
// command goes on, step 8's promotion replaces the edited file, and the
// files are kept in overwritten, which step 10 reports as lost edits.
//
// On an error each candidate of the run is declined, which deletes its
// dir, so a command that returns leaves no candidate dir (as step 6).
//
// A drifted kit dir is the one live path --overwrite-drift cannot
// replace: a kit dir is content-addressed and verified, never
// rewritten (01 1.6), so the promotion refuses it
// (render.ErrKitDirDiffers) and the command exits 1.
func checkDrift(_ context.Context, r *run) error {
	if err := driftOf(r); err != nil {
		return errors.Join(err, decline(r.candidates))
	}
	return nil
}

// driftOf is step 7 without the decline of the candidates on an error.
func driftOf(r *run) error {
	for i, p := range r.targets {
		drifted, err := render.CheckDrift(r.trees[i].env)
		if err != nil {
			return fmt.Errorf("the drift check of %s: %w", termsafe.Escape(p.Name), err)
		}
		if len(drifted) == 0 {
			continue
		}
		if !r.flags.overwriteDrift {
			names := make([]string, len(drifted))
			for j, d := range drifted {
				names[j] = termsafe.Escape(d)
			}
			e := cli.DriftDetected.New("the live file "+names[0]+" of "+termsafe.Escape(p.Name)+" differs from its render.json",
				cli.Detail{Key: "drifted", Value: strings.Join(names, ", ")})
			e.Fill = map[string]string{"<name>": p.Name}
			return e
		}
		for _, d := range drifted {
			r.overwritten = append(r.overwritten, lostEdit{project: p.Name, path: d})
		}
	}
	return nil
}

// lostEdit is a live file of a project that --overwrite-drift let step
// 8 replace, its edit lost.
type lostEdit struct {
	project, path string
}
