// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package j1 is the scenario function of J1, onboarding on a new machine
// (09, J1). It reads its command lines from commands.yaml (REQ-0730) and
// asserts each [H] step by the records it writes (REQ-0729). It is built
// up to step 3, romeu init; the steps after it are added by the tasks
// that add their commands, each in a file of its own (rule 1 of
// docs/plan.md). Steps 0 to 2 are [O] steps, preconditions of the
// scenario, and carry no assertion here.
package j1

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// ID is the scenario's id in commands.yaml.
const ID = "J1"

// Vars are the values J1 binds to the placeholders of its lines: the
// config repo's owner and name.
type Vars struct{ Owner, ConfigRepo string }

// J1 returns the scenario, its lines read from t.
func J1(t *scenarios.Table, v Vars) scenarios.Scenario {
	bind := scenarios.Vars{"owner": v.Owner, "config-repo": v.ConfigRepo}
	return scenarios.Scenario{
		ID: ID,
		Steps: []scenarios.Step{
			{N: "3", Kind: scenarios.Host, Line: bind.Bind(t.Line(ID, 0)), Assert: initWrote(v)},
		},
	}
}

// initWrote asserts step 3: romeu init wrote host settings with the
// root and absolute tool paths, and cloned the config repo to
// <root>/<cfg>-env/<dir> (04 4.2, REQ-0309).
func initWrote(v Vars) func(scenarios.World) error {
	return func(w scenarios.World) error {
		data, err := w.Settings()
		if err != nil {
			return fmt.Errorf("the host settings: %w", err)
		}
		h, err := spec.DecodeHostSettings(data)
		if err != nil {
			return fmt.Errorf("the host settings: %w", err)
		}
		if h.Root != w.Root() {
			return fmt.Errorf("root is %q, want %q", h.Root, w.Root())
		}
		for name, p := range map[string]string{"sbx": h.Tools.Sbx, "git": h.Tools.Git} {
			if !filepath.IsAbs(p) {
				return fmt.Errorf("tools.%s is %q, not an absolute path", name, p)
			}
		}
		if h.Config.Project != v.ConfigRepo {
			return fmt.Errorf("config.project is %q, want %q", h.Config.Project, v.ConfigRepo)
		}
		cloned := filepath.Join(h.Root, h.Config.Project+"-env", v.ConfigRepo, "projects", v.ConfigRepo+".yaml")
		if _, err := os.Stat(cloned); err != nil {
			return fmt.Errorf("the config repo is not cloned: %w", err)
		}
		return nil
	}
}
