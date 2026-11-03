// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package j9 is the scenario function of J9, a kit or workload bump (09,
// J9). It reads its command lines from commands.yaml (REQ-0730) and
// asserts each [H] step by the records it writes (REQ-0729). It holds
// the first step, romeu release: the operator runs romeu upgrade at a
// terminal and answers the one question it prints, Question. The world
// of a scenario runs a line without a terminal, so the e2e of the step
// (e2e/upgrade_test.go) runs the line on one and then calls the step's
// assertion; the steps after it are added by the tasks that add what
// they need, each in a file of its own (rule 1 of docs/plan.md).
package j9

import (
	"encoding/json/v2"
	"fmt"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios"
)

// ID is the scenario's id in commands.yaml.
const ID = "J9"

// Question is the prompt romeu upgrade prints last, before the answer:
// the question that ends the preview of the new binary (04 4.2,
// REQ-1348).
const Question = "install this romeu and acknowledge this toolchain? [y/N] "

// Vars are the values J9 checks its first step against: the sha256 of
// the new romeu, as toolchain.json records it once the operator
// answered y.
type Vars struct{ NewSHA256 string }

// J9 returns the scenario, its lines read from t.
func J9(t *scenarios.Table, v Vars) scenarios.Scenario {
	return scenarios.Scenario{
		ID: ID,
		Steps: []scenarios.Step{
			{N: "1", Kind: scenarios.Host, Line: t.Line(ID, 0), Assert: acknowledged(v)},
		},
	}
}

// acknowledged asserts step 1 after the answer y: the new romeu, in
// place of the process, recorded itself in toolchain.json with no
// second prompt (gate 1, 01 1.4, REQ-1330).
func acknowledged(v Vars) func(scenarios.World) error {
	return func(w scenarios.World) error {
		data, err := w.State("toolchain.json")
		if err != nil {
			return fmt.Errorf("toolchain.json: %w", err)
		}
		var rec struct {
			Romeu struct {
				SHA256 string `json:"sha256"`
			} `json:"romeu"`
		}
		if err := json.Unmarshal(data, &rec); err != nil {
			return fmt.Errorf("toolchain.json: %w", err)
		}
		if rec.Romeu.SHA256 != v.NewSHA256 {
			return fmt.Errorf("toolchain.json records romeu %s, want the new binary %s", rec.Romeu.SHA256, v.NewSHA256)
		}
		return nil
	}
}
