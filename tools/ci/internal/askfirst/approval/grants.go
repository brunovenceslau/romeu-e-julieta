// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package approval

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"

	"go.yaml.in/yaml/v3"
)

// GrantsPath is the file of the sitting grants (12 12.4), relative to
// the repository root. pr reads it at the base commit.
const GrantsPath = "docs/grants.yaml"

// Grantable are the surfaces a sitting may grant the approval lines of
// (12 12.4, REQ-1159 to REQ-1162); every other surface keeps a line the
// maintainer ticks on each pull request.
var Grantable = []string{"contracts", "checks", "dependencies", "catalog", "ledger", "digests", "termsafe"}

// sittingID is the id of a sitting of the plan: a milestone, M0 to M5,
// or an earlier operator block, O1 to O8 (docs/plan.md, the sitting
// field).
var sittingID = regexp.MustCompile(`^(M[0-5]|O[1-8])$`)

// Grants is docs/grants.yaml (REQ-1160).
type Grants struct {
	Version int     `yaml:"version"`
	Grants  []Grant `yaml:"grants"`
}

// Grant is one entry: at a sitting, the approval lines of tasks on one
// grantable surface.
type Grant struct {
	Sitting string   `yaml:"sitting"`
	Surface string   `yaml:"surface"`
	Tasks   []string `yaml:"tasks"`
}

// ParseGrants reads docs/grants.yaml and checks it against the task ids
// the plan defines (REQ-1160): the keys are its own, the version is 1,
// and every entry names a sitting, a grantable surface and tasks of the
// plan.
func ParseGrants(src []byte, tasks []string) (*Grants, error) {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	dec.KnownFields(true)
	var g Grants
	if err := dec.Decode(&g); err != nil {
		return nil, fmt.Errorf("%s: %w", GrantsPath, err)
	}
	var more yaml.Node
	if err := dec.Decode(&more); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: holds more than one YAML document", GrantsPath)
	}
	if g.Version != 1 {
		return nil, fmt.Errorf("%s: version is %d, and this tool reads version 1", GrantsPath, g.Version)
	}
	for _, e := range g.Grants {
		if !sittingID.MatchString(e.Sitting) {
			return nil, fmt.Errorf("%s: %q is not a sitting of the plan", GrantsPath, e.Sitting)
		}
		if !slices.Contains(Grantable, e.Surface) {
			return nil, fmt.Errorf("%s: a sitting grants no approval line on %q", GrantsPath, e.Surface)
		}
		if len(e.Tasks) == 0 {
			return nil, fmt.Errorf("%s: the grant of %s on %q names no task", GrantsPath, e.Sitting, e.Surface)
		}
		for _, t := range e.Tasks {
			if !slices.Contains(tasks, t) {
				return nil, fmt.Errorf("%s: the grant of %s on %q names %q, which the plan does not define", GrantsPath, e.Sitting, e.Surface, t)
			}
		}
	}
	return &g, nil
}

// holds reports whether a grant gives task the approval line of surface
// at sitting. A nil Grants holds none.
func (g *Grants) holds(sitting, surface, task string) bool {
	if g == nil {
		return false
	}
	for _, e := range g.Grants {
		if e.Sitting == sitting && e.Surface == surface && slices.Contains(e.Tasks, task) {
			return true
		}
	}
	return false
}
