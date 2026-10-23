// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render

import (
	"cmp"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"path"
	"regexp"
	"slices"
)

// The workspace files of $ROMEU_ROOT (02 2.3), each named relative to
// that directory.
const (
	// DevWorkspaceFile lists every host clone.
	DevWorkspaceFile = "dev.code-workspace"
	// ReviewWorkspaceFile lists every review checkout.
	ReviewWorkspaceFile = "review.code-workspace"
)

// reviewDir is the dir of the review checkouts in <name>-env/, a name
// romeu reserves (03 3.1).
const reviewDir = "review"

// Clone is one repo checkout of a project in $ROMEU_ROOT: a host clone
// at <project>-env/<dir>, or a review checkout at
// <project>-env/review/<dir>, as host state records it.
type Clone struct {
	// Project is the project's name.
	Project string
	// Dir is the repo's dir.
	Dir string
}

// workspaceFile is a VS Code workspace file holding folders alone (I13).
type workspaceFile struct {
	Folders []workspaceFolder `json:"folders"`
}

// workspaceFolder is one folder of a workspace file, by its path
// relative to the file.
type workspaceFolder struct {
	Path string `json:"path"`
}

// DevWorkspace returns dev.code-workspace (02 2.3, REQ-0137): one
// folder for each host clone, <project>-env/<dir>, sorted. A clone's dir
// is never a name romeu reserves, so neither a memory dir nor a review
// checkout can be a folder of it (I13).
//
//romeu:invariant I13 guard
func DevWorkspace(clones []Clone) ([]byte, error) {
	return workspace(clones, func(c Clone) string { return path.Join(c.Project+"-env", c.Dir) })
}

// ReviewWorkspace returns review.code-workspace (02 2.3, REQ-0137): one
// folder for each review checkout, <project>-env/review/<dir>, sorted.
// It is the one file a review checkout is a folder of, and a memory dir
// is never one (I13).
//
//romeu:invariant I13 guard
func ReviewWorkspace(checkouts []Clone) ([]byte, error) {
	return workspace(checkouts, func(c Clone) string { return path.Join(c.Project+"-env", reviewDir, c.Dir) })
}

// workspace writes the workspace file of clones, each folder at the
// path folderOf returns, through the JSON marshaller (I11), with the
// key folders alone (I13).
func workspace(clones []Clone, folderOf func(Clone) string) ([]byte, error) {
	sorted := slices.SortedFunc(slices.Values(clones), func(a, b Clone) int {
		return cmp.Or(cmp.Compare(a.Project, b.Project), cmp.Compare(a.Dir, b.Dir))
	})
	f := workspaceFile{Folders: []workspaceFolder{}}
	for i, c := range sorted {
		if !projectName.MatchString(c.Project) {
			return nil, fmt.Errorf("render: %q is not a project name", c.Project)
		}
		if err := oneElement(c.Dir); err != nil {
			return nil, fmt.Errorf("render: the project %q: %w", c.Project, err)
		}
		if i > 0 && sorted[i-1] == c {
			return nil, fmt.Errorf("render: the project %q lists the dir %q twice", c.Project, c.Dir)
		}
		f.Folders = append(f.Folders, workspaceFolder{Path: folderOf(c)})
	}
	b, err := jsonv2.Marshal(f, jsonv2.Deterministic(true), jsontext.WithIndent("  "))
	if err != nil {
		return nil, fmt.Errorf("render: encode a workspace file: %w", err)
	}
	return append(b, '\n'), nil
}

// projectName is the project name rule of 03 3.1, from internal/spec,
// the one place it is written.
var projectName = regexp.MustCompile(specRule("project-name").Pattern)
