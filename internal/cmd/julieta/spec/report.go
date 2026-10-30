// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"encoding/json/v2"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	projectspec "github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// Schema is the value of the schema field of the --json document of
// julieta spec validate, the format julieta-spec.v1.
const Schema = "julieta-spec/v1"

// Report is the --json document of julieta spec validate, the format
// julieta-spec.v1 (04 4.1, Output; 04 4.3, REQ-0422): what the text
// output says, one member per kind of line. The fields are in the
// order of their JSON names.
type Report struct {
	// Files are the paths judged, in order: the files given, then the
	// entries of projects/ the command added (REQ-0423).
	Files []string `json:"files"`
	// Findings are the findings, in the order of the text output; any
	// one makes the command exit 6.
	Findings []Finding `json:"findings"`
	// NotChecked are the slugs of the rules of 03 3.1 only romeu
	// checks, since they read host settings (REQ-0174).
	NotChecked []string `json:"notChecked"`
	// Schema is "julieta-spec/v1".
	Schema string `json:"schema"`
	// Warnings are the warnings of the run, such as lock-host-gated.
	Warnings []setup.Warning `json:"warnings"`
}

// Finding is one finding: the slug of its rule and the text that names
// the file, the place and why.
type Finding struct {
	// Rule is the slug of the rule.
	Rule string `json:"rule"`
	// Text names the file, the place of the value and why it is
	// refused.
	Text string `json:"text"`
}

// newReport returns the report of the files judged, the findings and
// the warnings.
func newReport(files []projectspec.File, findings []finding, warnings []setup.Warning) Report {
	rep := Report{Files: []string{}, Findings: []Finding{}, NotChecked: []string{}, Schema: Schema, Warnings: []setup.Warning{}}
	for _, f := range files {
		rep.Files = append(rep.Files, f.Path)
	}
	for _, f := range findings {
		rep.Findings = append(rep.Findings, Finding{Rule: f.rule, Text: f.text})
	}
	for _, r := range projectspec.HostOnly() {
		rep.NotChecked = append(rep.NotChecked, r.Slug)
	}
	rep.Warnings = append(rep.Warnings, warnings...)
	return rep
}

// writeJSON prints the report as one JSON document on stdout.
func writeJSON(env *cli.Env, rep Report) error {
	b, err := json.Marshal(rep, json.Deterministic(true))
	if err != nil {
		return err
	}
	_, err = env.Stdout.Write(append(b, '\n'))
	return err
}
