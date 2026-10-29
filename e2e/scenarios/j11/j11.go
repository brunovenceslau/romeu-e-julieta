// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package j11 is the scenario function of J11, an egress or secret
// change approval (09, J11). It reads its command lines from
// commands.yaml (REQ-0730) and asserts each [H] step by the records it
// writes (REQ-0729). It is built for step 5, a sync without a terminal
// of a project whose widening changed; steps 2 to 4 are added by the
// tasks that add what they need, step 3's live egress among them
// (T047), each in a file of its own (rule 1 of docs/plan.md).
//
// The world of a scenario runs a line without a terminal, which is the
// case of step 5; the rerun on a terminal that step 5 then names is the
// pty case of e2e/sync_test.go.
package j11

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios"
)

// ID is the scenario's id in commands.yaml.
const ID = "J11"

// Vars are the values J11 binds to the placeholders of its lines: the
// project's name, and its live sbxenv.yaml before the line, which the
// line must leave as it was.
type Vars struct {
	Name string
	Live []byte
}

// J11 returns the scenario, its lines read from t.
func J11(t *scenarios.Table, v Vars) scenarios.Scenario {
	bind := scenarios.Vars{"name": v.Name}
	return scenarios.Scenario{
		ID: ID,
		Steps: []scenarios.Step{
			{N: "5", Kind: scenarios.Host, Line: bind.Bind(t.Line(ID, 0)), Want: 3, Assert: nothingKept(v)},
		},
	}
}

// nothingKept asserts step 5: without a terminal, a sync whose widening
// digest differs from the approval exits 3 with nothing kept (01 1.6,
// REQ-1210): no candidate dir is left, and the live sbxenv.yaml is the
// one approved before.
func nothingKept(v Vars) func(scenarios.World) error {
	return func(w scenarios.World) error {
		env := filepath.Join(w.Root(), v.Name+"-env")
		entries, err := os.ReadDir(filepath.Join(env, ".romeu", "candidates"))
		if err != nil {
			return fmt.Errorf("the candidates dir: %w", err)
		}
		if len(entries) != 0 {
			return fmt.Errorf("a candidate dir is left: %s", entries[0].Name())
		}
		live, err := os.ReadFile(filepath.Join(env, "sbxenv.yaml"))
		if err != nil {
			return fmt.Errorf("the live sbxenv.yaml: %w", err)
		}
		if !bytes.Equal(live, v.Live) {
			return fmt.Errorf("the live sbxenv.yaml changed without an approval")
		}
		return nil
	}
}
