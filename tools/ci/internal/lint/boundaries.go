// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lint

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// BoundaryConfig is the configuration of the second run, forbidigo
// alone, by its path from the root of the repository.
const BoundaryConfig = "tools/ci/internal/lint/forbidigo.yml"

// Allowed are the packages, by directory from the root, that the
// output patterns of BoundaryConfig do not judge, tests included. 04 4.1
// names no set of packages for them, so this list is a decision of the
// task that made it, open for the operator to confirm. It is derived
// from REQ-0269 (all output goes through the CLI writer) and the
// composition roots:
//
//   - internal/cli: the CLI writer, which owns the output streams, and
//     the capture of os.Stderr around cobra.
//   - cmd/romeu, cmd/julieta, tools/ci, tools/ci/internal/askfirst/cmd/ask-first
//     (the command of the ask-first job, which imports no writer of tools/ci),
//     tools/kitpin, tools/manifest, tools/mutants, tools/new,
//     tools/planviewer and tools/release: the main packages,
//     which hand
//     os.Stdout and os.Stderr to the writer of their tool and hold the
//     dispatcher and nothing else.
//   - e2e/fakesbx: the fake sbx, a main package whose output is sbx's
//     own, replayed from a session, which no CLI writer of the product
//     is in the way of.
//   - internal/procgroup: whose tests run helper processes that write to
//     the streams of the process they are.
//
// The list is exact, a directory and not a tree: tools/ci/internal/...
// are checks, and are held to the patterns.
var Allowed = []string{
	"cmd/julieta",
	"cmd/romeu",
	"e2e/fakesbx",
	"internal/cli",
	"internal/procgroup",
	"tools/ci",
	"tools/ci/internal/askfirst/cmd/ask-first",
	"tools/kitpin",
	"tools/manifest",
	"tools/mutants",
	"tools/new",
	"tools/planviewer",
	"tools/release",
}

// BoundaryDirs returns the directories under root, as "./dir" and in
// order, that hold Go files and are not in Allowed: the packages the
// second run judges. It leaves out testdata and hidden directories, as
// the go command does, and the directories of other modules. Skipping a
// nested module relies on the nestedModules check of fast and all
// (tools/ci/fast.go), which refuses a go.mod below the root, so no
// package hides in one.
func BoundaryDirs(root string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		name := d.Name()
		if rel != "." && (name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
			return filepath.SkipDir
		}
		if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil && rel != "." {
			return filepath.SkipDir
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			return err
		}
		hasGo := slices.ContainsFunc(entries, func(e fs.DirEntry) bool {
			return !e.IsDir() && path.Ext(e.Name()) == ".go"
		})
		switch {
		case !hasGo || slices.Contains(Allowed, rel):
		case rel == ".":
			dirs = append(dirs, ".")
		default:
			dirs = append(dirs, "./"+rel)
		}
		return nil
	})
	return dirs, err
}

// BoundaryArgs are the arguments of the second run for the module at
// root: Args with BoundaryConfig for the config, then BoundaryDirs. It
// returns nil when no package is left to judge.
func BoundaryArgs(root string) ([]string, error) {
	dirs, err := BoundaryDirs(root)
	if err != nil || len(dirs) == 0 {
		return nil, err
	}
	args := slices.Clone(Args)
	at := slices.Index(args, ".golangci.yml")
	args[at] = BoundaryConfig
	return append(args, dirs...), nil
}
