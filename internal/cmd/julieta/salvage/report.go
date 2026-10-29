// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage

import (
	"encoding/json/v2"
	"fmt"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

// Schema is the value of the schema member of the --json document of
// julieta salvage, the format julieta-salvage.v1.
const Schema = "julieta-salvage/v1"

// Report is the --json document of julieta salvage, the format
// julieta-salvage.v1: the fingerprint object of the reads (REQ-0417)
// and each manifest written, with its path and repo (04 4.1,
// Idempotency, REQ-0280). Completeness is the host's verdict (08 8.5);
// each manifest's complete is advisory. The fields are in the order of
// their JSON names.
type Report struct {
	// Fingerprint is the fingerprint object of 03 3.8, the one
	// --fingerprint prints.
	Fingerprint salvage.Fingerprint `json:"fingerprint"`
	// Manifests are the manifests written, one per repo.
	Manifests []ReportManifest `json:"manifests"`
	// SalvageID is the salvage id of the manifest's salvageRun.
	SalvageID string `json:"salvageId"`
	// Schema is "julieta-salvage/v1".
	Schema string `json:"schema"`
	// SecretBearing says that a payload holds ignored files or agent
	// state, which can hold credentials and code (REQ-0710).
	SecretBearing bool `json:"secretBearing"`
}

// ReportManifest is one manifest a salvage wrote.
type ReportManifest struct {
	// Complete is the manifest's advisory complete.
	Complete bool `json:"complete"`
	// Path is the manifest's path in the sandbox.
	Path string `json:"path"`
	// Repo is the repo's dir.
	Repo string `json:"repo"`
	// Skipped is the number of items it left out.
	Skipped int `json:"skipped"`
}

// secretLine is what the text output says when a payload holds ignored
// files or agent state (REQ-0710).
const secretLine = "Salvage payloads are secret-bearing: ignored files and transcripts can hold credentials and code."

// report prints what the salvage wrote: the --json document, or one
// line per manifest with its repo and what it left out.
func report(env *cli.Env, id string, res salvage.Result) error {
	rep := Report{Fingerprint: res.Fingerprint, Manifests: []ReportManifest{}, SalvageID: id, Schema: Schema, SecretBearing: res.SecretBearing}
	for _, w := range res.Manifests {
		rep.Manifests = append(rep.Manifests, ReportManifest{Complete: w.Manifest.Complete, Path: w.Path, Repo: w.Repo, Skipped: len(w.Manifest.Skipped)})
	}
	if env.JSON {
		return writeJSON(env, rep)
	}
	for _, w := range res.Manifests {
		if _, err := fmt.Fprintf(env.Stdout, "wrote %s repo %s salvage %s\n", w.Path, w.Repo, id); err != nil {
			return err
		}
		for _, s := range w.Manifest.Skipped {
			line := "skipped " + s.Reason + " " + s.Item
			if s.Detail != "" {
				line += ": " + s.Detail
			}
			if _, err := fmt.Fprintln(env.Stdout, line); err != nil {
				return err
			}
		}
	}
	if res.SecretBearing {
		_, err := fmt.Fprintln(env.Stdout, secretLine)
		return err
	}
	return nil
}

// writeJSON prints v as one line of JSON, its members sorted.
func writeJSON(env *cli.Env, v any) error {
	b, err := json.Marshal(v, json.Deterministic(true))
	if err != nil {
		return err
	}
	_, err = env.Stdout.Write(append(b, '\n'))
	return err
}
