// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// Schema is the value of the schema field of the --json document of
// romeu doctor, the format romeu-doctor.v1.
const Schema = "romeu-doctor/v1"

// Results are the values of the result of a check, in the order of 04
// 4.2 (REQ-0331): the schema of romeu-doctor.v1 admits these alone.
var Results = []string{resultPass, resultWarn, resultFail, resultError}

// Report is the --json document of romeu doctor, the format
// romeu-doctor.v1 (04 4.1, Output; 04 4.2, REQ-0331): one object per
// check, in slug order, and the leftover temporary files of the state
// directory. The fields are in the order of their JSON names.
type Report struct {
	// Checks are the results, one per registered check.
	Checks []Check `json:"checks"`
	// Leftovers are the temporary files an interrupted atomic write left
	// in the state directory (03 3.8), for the operator to remove by
	// hand: doctor lists them and deletes none (T049, re-plan of
	// 2026-10-22). They are no check, so they change no exit code.
	Leftovers []string `json:"leftovers"`
	// Schema is "romeu-doctor/v1".
	Schema string `json:"schema"`
}

// Check is the result of one check (04 4.2, REQ-0331).
type Check struct {
	// Slug is the check's slug, a row of the table of 04 4.1.
	Slug string `json:"slug"`
	// Result is pass, warn, fail or error.
	Result string `json:"result"`
	// ID and Error are, for the result error, the id and the slug of the
	// error that kept the check from running (04 4.4).
	ID    string `json:"id,omitzero"`
	Error string `json:"error,omitzero"`
	// Message says what the check found, or why it could not run.
	Message string `json:"message,omitzero"`
}

// run runs every registered check over h, prints the report to
// env.Stdout, as text or with env.JSON as one document, and returns the
// error of the exit of 04 4.2 (REQ-0331): doctor-incomplete (exit 1)
// when a check could not run, check-findings (exit 6) when every check
// ran and one failed, nil when none did. A warning changes no exit.
func run(ctx context.Context, env *cli.Env, h *host) error {
	rep := Report{Checks: []Check{}, Leftovers: leftovers(h), Schema: Schema}
	for _, r := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		c := r.run(ctx, h)
		// A byte that is not UTF-8 would stop the JSON encoder and lose
		// the whole report.
		c.Message = strings.ToValidUTF8(c.Message, "�")
		rep.Checks = append(rep.Checks, c)
	}
	rep.Checks = append(rep.Checks, h.unplaced(ctx)...)
	if err := write(env, rep); err != nil {
		return err
	}
	var errored, failed []cli.Detail
	for _, c := range rep.Checks {
		switch c.Result {
		case resultError:
			errored = append(errored, cli.Detail{Key: c.Slug, Value: c.ID + " " + c.Error})
		case resultFail:
			failed = append(failed, cli.Detail{Key: c.Slug, Value: line(c.Message)})
		}
	}
	switch {
	case len(errored) > 0:
		return DoctorIncomplete.New(fmt.Sprintf("%d check(s) could not run, so the report is incomplete; %d failed", len(errored), len(failed)),
			append(errored, failed...)...)
	case len(failed) > 0:
		return cli.CheckFindings.New(fmt.Sprintf("%d check(s) failed", len(failed)), failed...)
	}
	return ctx.Err()
}

// write prints rep: one JSON document with env.JSON, else one line per
// check, "<result> <slug>", then what it found, and a line per leftover
// file.
func write(env *cli.Env, rep Report) error {
	if env.JSON {
		b, err := json.Marshal(rep, json.Deterministic(true))
		if err != nil {
			return err
		}
		_, err = env.Stdout.Write(append(b, '\n'))
		return err
	}
	var b strings.Builder
	for _, c := range rep.Checks {
		fmt.Fprintf(&b, "%-5s %s", c.Result, c.Slug)
		if c.Result == resultError {
			fmt.Fprintf(&b, " (%s %s)", c.ID, c.Error)
		}
		if c.Message != "" {
			fmt.Fprintf(&b, ": %s", line(c.Message))
		}
		b.WriteString("\n")
	}
	for _, p := range rep.Leftovers {
		fmt.Fprintf(&b, "leftover temporary file, remove it by hand: %s\n", line(p))
	}
	_, err := env.Stdout.Write([]byte(b.String()))
	return err
}

// line returns s with each line feed written as the two characters \n,
// so a value, such as an error that holds a subprocess's stderr, stays
// on its line and cannot start one that forges another; the CLI writer
// escapes every other code point (I29).
func line(s string) string { return strings.ReplaceAll(s, "\n", `\n`) }
