// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package j2 is the scenario function of J2, a new project (09, J2). It
// reads its command lines from commands.yaml (REQ-0730) and asserts each
// [H] step by the records it writes (REQ-0729). It is built up to step
// 3, the host half of romeu sync; steps 1 and 4 are added by the tasks
// that add their commands, each in a file of its own (rule 1 of
// docs/plan.md). Step 2 is an [O] step, the command line itself, and
// carries no assertion of its own.
//
// The world of a scenario runs a line without a terminal, so gate 2
// cannot be approved there: the line ends at gate 2 with
// project-approval-required (exit 3), once the tree of step 3 is made,
// and the approval on a terminal is the pty case of e2e/sync_test.go.
package j2

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios"
)

// ID is the scenario's id in commands.yaml.
const ID = "J2"

// Vars are the values J2 binds to the placeholders of its lines: the
// project's name, and the dirs of its repos, which step 3 clones.
type Vars struct {
	Name string
	Dirs []string
}

// J2 returns the scenario, its lines read from t.
func J2(t *scenarios.Table, v Vars) scenarios.Scenario {
	bind := scenarios.Vars{"name": v.Name}
	return scenarios.Scenario{
		ID: ID,
		Steps: []scenarios.Step{
			{N: "3", Kind: scenarios.Host, Line: bind.Bind(t.Line(ID, 0)), Want: 3, Assert: treeMade(v)},
		},
	}
}

// treeMade asserts step 3 without a terminal (04 4.2, steps 4 to 8):
// <name>-env/ made 0700 with its .metadata_never_index marker and its
// memory dirs, each repo cloned, and gate 2 reached: the candidate dir
// deleted, nothing promoted and no approval recorded (01 1.6,
// REQ-1210).
func treeMade(v Vars) func(scenarios.World) error {
	return func(w scenarios.World) error {
		env := filepath.Join(w.Root(), v.Name+"-env")
		private := []string{env, filepath.Join(env, "memory", "_shared")}
		for _, d := range v.Dirs {
			private = append(private, filepath.Join(env, "memory", d), filepath.Join(env, "memory", d, "entries"))
			if info, err := os.Lstat(filepath.Join(env, d, ".git")); err != nil || !info.IsDir() {
				return fmt.Errorf("step 3 did not clone %s: %v", d, err)
			}
		}
		for _, d := range private {
			info, err := os.Lstat(d)
			if err != nil {
				return fmt.Errorf("step 3 did not make %s: %w", d, err)
			}
			if !info.IsDir() || info.Mode().Perm() != 0o700 {
				return fmt.Errorf("%s is %v, want a directory of mode 0700", d, info.Mode())
			}
		}
		if _, err := os.Lstat(filepath.Join(env, ".metadata_never_index")); err != nil {
			return fmt.Errorf("the marker: %w", err)
		}
		entries, err := os.ReadDir(filepath.Join(env, ".romeu", "candidates"))
		if err != nil {
			return fmt.Errorf("the candidates dir: %w", err)
		}
		if len(entries) != 0 {
			return fmt.Errorf("a candidate dir is left: %s", entries[0].Name())
		}
		if _, err := os.Lstat(filepath.Join(env, "sbxenv.yaml")); !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("sbxenv.yaml is live without an approval: %v", err)
		}
		if _, err := w.State(filepath.Join("projects", v.Name+".json")); !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("a project record is written without an approval: %v", err)
		}
		return nil
	}
}
