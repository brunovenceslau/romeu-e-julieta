// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package moduletree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// TestModuleTreeOfThisRepository holds this repository's tree, module
// table, coverTrees and packages under internal/ to one set.
func TestModuleTreeOfThisRepository(t *testing.T) {
	findings, err := Run(citest.ModuleRoot(t))
	require.NoError(t, err)
	assert.Empty(t, findings)
}

// The pages of a fixture that passes.
const (
	fixtureTree = "# 2\n\n## 2.1 Product repo\n\n```\nroot/\n" +
		"├─ go.mod                      module\n" +
		"├─ internal/\n" +
		"│  ├─ canon/                   digests\n" +
		"│  ├─ memstore/                readers\n" +
		"│  ├─ memstore/write/          writes\n" +
		"│  ├─ cmd/romeu/<command>/     one package per command\n" +
		"│  └─ cli/                     the dispatcher\n" +
		"├─ tools/\n" +
		"│  └─ kitpin/                  pins\n" +
		"```\n\n## 2.2 Next\n\n```\n├─ internal/\n│  └─ other/\n```\n"
	fixtureModules = "# 12\n\n## 12.2 Modules\n\n" +
		"| Module id | Responsibility | Depends on |\n|---|---|---|\n" +
		"| `ci-bootstrap` | `tools/ci` | - |\n" +
		"| `kitpin` | `tools/kitpin` | - |\n" +
		"| `canon` | digests | - |\n" +
		"| `memstore` | readers and `memstore/write` | canon |\n" +
		"| `romeu-cli` | commands under `internal/cmd/romeu/` | all |\n" +
		"| `cli` | the dispatcher | - |\n" +
		"\nAfter the table.\n"
	fixtureCoverage = "package coverage\n\nvar coverTrees = []string{\"cmd\", \"internal\", \"tools\"}\n"
)

// newFixture writes a repository that passes, with the files given on
// top ("" removes one), and returns its root.
func newFixture(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	all := []string{
		treePage, fixtureTree,
		modulePage, fixtureModules,
		"tools/ci/internal/coverage/coverage.go", fixtureCoverage,
		"tools/ci/internal/coverage/coverage_test.go", "package coverage\n\nvar coverTrees = []string{}\n",
		"internal/canon/canon.go", "package canon\n",
		"internal/canon/testdata/x.go", "package x\n",
		"internal/canon/README.md", "not Go\n",
		"internal/memstore/write/write.go", "package write\n",
		"internal/cmd/romeu/init/init.go", "package init\n",
		"internal/cli/cli.go", "package cli\n",
	}
	all = append(all, files...)
	for i := 0; i < len(all); i += 2 {
		p := filepath.Join(root, all[i])
		if all[i+1] == "" {
			require.NoError(t, os.Remove(p))
			continue
		}
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(all[i+1]), 0o644))
	}
	return root
}

// TestModuleTreeFindings covers each rule, a fixture each way.
func TestModuleTreeFindings(t *testing.T) {
	tests := []struct {
		name    string
		files   []string
		want    []string
		prepare func(t *testing.T, root string)
	}{
		{"a fixture that passes", nil, nil, nil},
		{"a package under internal/ with no module row", []string{
			treePage, strings.Replace(fixtureTree, "│  └─ cli/", "│  ├─ extra/                   x\n│  └─ cli/", 1),
			"internal/extra/extra.go", "package extra\n",
		}, []string{
			"internal/extra: a package no row of the module table of 12 12.2 owns",
			"internal/extra: in the tree of 02 2.1, and no row of the module table of 12 12.2 owns it",
		}, nil},
		{"a module row with no directory", []string{
			modulePage, strings.Replace(fixtureModules, "\nAfter", "| `ghost` | nothing | - |\n\nAfter", 1),
		}, []string{"module ghost: names no directory of the tree of 02 2.1"}, nil},
		{"a module id with a hyphen is no directory name", []string{
			modulePage, strings.Replace(fixtureModules, "\nAfter", "| `ci-release` | the rest | - |\n\nAfter", 1),
		}, nil, nil},
		{"a row that gives a path the tree lacks", []string{
			modulePage, strings.Replace(fixtureModules, "under `internal/cmd/romeu/`", "under `internal/cmd/romeu/` and `internal/preflight`", 1),
		}, []string{"module romeu-cli: gives internal/preflight, which the tree of 02 2.1 lacks"}, nil},
		{"a package below a directory of the tree, and one the tree lacks", []string{
			"internal/canon/inner/inner.go", "package inner\n",
			"internal/stray/stray.go", "package stray\n",
		}, []string{
			"internal/stray: a package no row of the module table of 12 12.2 owns",
			"internal/stray: a package the tree of 02 2.1 lacks",
		}, nil},
		{"internal/ below none of coverTrees", []string{
			"tools/ci/internal/coverage/coverage.go", strings.Replace(fixtureCoverage, `"internal", `, "", 1),
		}, []string{
			"internal/canon: below none of coverTrees, so S9 does not measure it",
			"internal/cli: below none of coverTrees, so S9 does not measure it",
			"internal/cmd/romeu/<command>: below none of coverTrees, so S9 does not measure it",
			"internal/cmd/romeu/init: below none of coverTrees, so S9 does not measure it",
			"internal/memstore/write: below none of coverTrees, so S9 does not measure it",
			"internal/memstore: below none of coverTrees, so S9 does not measure it",
		}, nil},
		{"a catch-all package name", []string{
			treePage, strings.Replace(fixtureTree, "│  └─ cli/", "│  ├─ canon/utils/              x\n│  └─ cli/", 1),
			"internal/canon/utils/utils.go", "package utils\n",
		}, []string{`internal/canon/utils: "utils" names no purpose; a package does one thing (10 10.6)`}, nil},
		{"each catch-all name", []string{
			"internal/canon/util/a.go", "package util\n",
			"internal/canon/helper/a.go", "package helper\n",
			"internal/canon/helpers/a.go", "package helpers\n",
			"internal/canon/common/a.go", "package common\n",
		}, []string{
			`internal/canon/common: "common" names no purpose; a package does one thing (10 10.6)`,
			`internal/canon/helper: "helper" names no purpose; a package does one thing (10 10.6)`,
			`internal/canon/helpers: "helpers" names no purpose; a package does one thing (10 10.6)`,
			`internal/canon/util: "util" names no purpose; a package does one thing (10 10.6)`,
		}, nil},
		{"no internal/ directory", nil, nil, func(t *testing.T, root string) {
			require.NoError(t, os.RemoveAll(filepath.Join(root, "internal")))
		}},
		{"a row path that owns a directory", []string{
			modulePage, strings.Replace(fixtureModules, "commands under `internal/cmd/romeu/`", "commands", 1),
		}, []string{
			"internal/cmd/romeu/<command>: in the tree of 02 2.1, and no row of the module table of 12 12.2 owns it",
			"internal/cmd/romeu/init: a package no row of the module table of 12 12.2 owns",
		}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newFixture(t, tt.files...)
			if tt.prepare != nil {
				tt.prepare(t, root)
			}
			findings, err := Run(root)
			require.NoError(t, err)
			assert.Equal(t, tt.want, findings)
		})
	}
}

// TestModuleTreeErrors covers the inputs it cannot read.
func TestModuleTreeErrors(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  string
	}{
		{"no tree page", []string{treePage, ""}, "02-layouts.md"},
		{"no section 2.1", []string{treePage, "# 2\n"}, "no section 2.1"},
		{"no fenced block", []string{treePage, "# 2\n\n## 2.1 Product\n\nText.\n"}, "no fenced block"},
		{"an open fenced block", []string{treePage, "# 2\n\n## 2.1 Product\n\n```\n├─ a/\n"}, "is not closed"},
		{"a line indented below no directory", []string{treePage, "# 2\n\n## 2.1 Product\n\n```\n│  │  ├─ a/\n```\n"}, "indented below no directory"},
		{"no module page", []string{modulePage, ""}, "12-engineering.md"},
		{"no module table", []string{modulePage, "# 12\n"}, "no module table"},
		{"a module table with no row", []string{modulePage, "# 12\n\n| Module id | R |\n|---|---|\n"}, "has no row"},
		{"no coverTrees", []string{"tools/ci/internal/coverage/coverage.go", "package coverage\n"}, "0 declarations of coverTrees"},
		{"coverTrees twice", []string{"tools/ci/internal/other/other.go", fixtureCoverage}, "2 declarations of coverTrees"},
		{"coverTrees not a literal", []string{"tools/ci/internal/coverage/coverage.go", "package coverage\n\nvar coverTrees = trees()\n"}, "not a literal"},
		{"coverTrees with a name in it", []string{"tools/ci/internal/coverage/coverage.go", "package coverage\n\nconst c = \"x\"\n\nvar coverTrees = []string{c}\n"}, "not a string literal"},
		{"a Go file that does not parse", []string{"tools/ci/internal/coverage/broken.go", "package"}, "expected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Run(newFixture(t, tt.files...))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// TestModuleTreeCoverTreesSkipsWhatGoSkips checks that testdata and the
// directories go ignores hold no coverTrees that counts.
func TestModuleTreeCoverTreesSkipsWhatGoSkips(t *testing.T) {
	root := newFixture(t,
		"tools/ci/internal/coverage/testdata/x.go", fixtureCoverage,
		"tools/ci/.hidden/x.go", fixtureCoverage,
		"tools/ci/_skip/x.go", fixtureCoverage,
	)
	trees, err := CoverTrees(filepath.Join(root, toolsCI))
	require.NoError(t, err)
	assert.Equal(t, []string{"cmd", "internal", "tools"}, trees)
}

// TestModuleTreeDirsOfTheTree checks the paths the tree's indent gives,
// a line that names two levels included.
func TestModuleTreeDirsOfTheTree(t *testing.T) {
	dirs, err := TreeDirs([]byte(fixtureTree))
	require.NoError(t, err)
	assert.Equal(t, []string{
		"internal", "internal/canon", "internal/memstore", "internal/memstore/write",
		"internal/cmd/romeu/<command>", "internal/cli", "tools", "tools/kitpin",
	}, dirs)
}

// TestModuleTreeDirsUnderALastEntry checks the depth of a line indented
// under a last entry ("└─"), whose indent is spaces, not "│".
func TestModuleTreeDirsUnderALastEntry(t *testing.T) {
	page := "## 2.1 Tree\n\n```\nroot/\n" +
		"├─ a/\n" +
		"│  └─ b/\n" +
		"│     └─ c/\n" +
		"└─ internal/\n" +
		"   ├─ canon/\n" +
		"   │  └─ inner/\n" +
		"   └─ cli/\n" +
		"```\n"
	dirs, err := TreeDirs([]byte("# 2\n\n" + page))
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "a/b", "a/b/c", "internal", "internal/canon", "internal/canon/inner", "internal/cli"}, dirs)
}

// TestModuleTreePrefix checks the segment match and its placeholders.
func TestModuleTreePrefix(t *testing.T) {
	assert.True(t, prefix("internal/cmd/romeu", "internal/cmd/romeu/<command>"))
	assert.True(t, prefix("internal/cmd/romeu/init", "internal/cmd/romeu/<command>"))
	assert.True(t, prefix("internal/cmd/<binary>", "internal/cmd/julieta/x"))
	assert.False(t, prefix("internal/cmd/romeu/init/x", "internal/cmd/romeu/<command>"))
	assert.False(t, prefix("internal/canonx", "internal/canon"))
	assert.False(t, placeholder("<>"))
}
