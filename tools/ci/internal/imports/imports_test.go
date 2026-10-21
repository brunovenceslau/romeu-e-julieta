// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package imports

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// pkg is one package of a fixture module: what its non-test file, its
// test file and its test file of the e2e suite (//go:build e2e) import,
// each path relative to the module, except a standard library path such
// as text/template.
type pkg struct {
	imports, testImports, e2eTestImports []string
}

// baseTree is a module that keeps every rule: both binaries, a command
// package of each, a package of each row, and tools/ci, tools/new and
// an e2e package importing what REQ-0971 lets them.
func baseTree() map[string]pkg {
	return map[string]pkg{
		"cmd/romeu":                  {imports: []string{"internal/cmd/romeu/all", "internal/cli", "internal/procgroup"}},
		"cmd/julieta":                {imports: []string{"internal/cmd/julieta/all", "internal/cli"}},
		"internal/cmd/romeu/all":     {imports: []string{"internal/cmd/romeu/version"}},
		"internal/cmd/romeu/version": {imports: []string{"internal/cli", "internal/signing"}},
		"internal/cmd/julieta/all":   {imports: []string{"internal/cmd/julieta/version"}},
		"internal/cmd/julieta/version": {imports: []string{"internal/cli", "internal/layout", "internal/hooks",
			"internal/mise", "internal/memstore/write", "internal/agent/claude"}},
		"internal/cli":             {imports: []string{"internal/termsafe"}, testImports: []string{"internal/cmd/romeu/all", "internal/cli/testsupport"}},
		"internal/cli/clidoc":      {imports: []string{"internal/cli"}},
		"internal/cli/testsupport": {imports: []string{"internal/cli"}},
		"internal/termsafe":        {},
		"internal/procgroup":       {},
		"internal/canon":           {},
		"internal/signing":         {imports: []string{"internal/canon"}},
		"internal/render":          {imports: []string{"encoding/json/v2"}},
		"internal/layout":          {},
		"internal/hooks":           {},
		"internal/mise":            {},
		"internal/memstore":        {},
		"internal/memstore/write":  {imports: []string{"internal/memstore"}},
		"internal/agent/claude":    {imports: []string{"internal/hooks"}},
		"tools/ci": {imports: []string{"internal/procgroup", "internal/cli/clidoc", "internal/cmd/romeu/all", "internal/cmd/julieta/all"},
			testImports: []string{"internal/procgroup"}},
		"tools/ci/internal/x": {testImports: []string{"internal/cli/clidoc"}},
		"tools/new":           {imports: []string{"internal/cli", "internal/procgroup"}},
		"e2e/x":               {testImports: []string{"internal/cmd/julieta/all", "internal/procgroup"}},
	}
}

// importLine returns the import line of p, a path as pkg names it.
func importLine(p string) string {
	if strings.HasPrefix(p, "internal/") || strings.HasPrefix(p, "cmd/") || strings.HasPrefix(p, "pkg/") {
		p = citest.FixtureModule + "/" + p
	}
	return fmt.Sprintf("import _ %q\n", p)
}

// writeTree writes the fixture module of pkgs to a new directory and
// returns it: one file per package, and one test file for those whose
// tests import something.
func writeTree(t *testing.T, pkgs map[string]pkg) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module "+citest.FixtureModule+"\n\ngo 1.22\n"), 0o600))
	for p, spec := range pkgs {
		name := path.Base(p)
		if strings.HasPrefix(p, "cmd/") {
			name = "main"
		}
		src := "package " + name + "\n\n"
		for _, imp := range spec.imports {
			src += importLine(imp)
		}
		if name == "main" {
			src += "\nfunc main() {}\n"
		}
		pkgDir := filepath.Join(dir, filepath.FromSlash(p))
		require.NoError(t, os.MkdirAll(pkgDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "p.go"), []byte(src), 0o600))
		if len(spec.testImports) > 0 {
			test := "package " + name + "\n\n"
			for _, imp := range spec.testImports {
				test += importLine(imp)
			}
			require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "p_test.go"), []byte(test), 0o600))
		}
		if len(spec.e2eTestImports) > 0 {
			test := "//go:build e2e\n\npackage " + name + "\n\n"
			for _, imp := range spec.e2eTestImports {
				test += importLine(imp)
			}
			require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "p_e2e_test.go"), []byte(test), 0o600))
		}
	}
	return dir
}

// goCommand returns the go command of the tests.
func goCommand(t *testing.T) string {
	t.Helper()
	goBin, err := procgroup.LookPath("go")
	require.NoError(t, err)
	return goBin
}

// loadTree writes the base tree changed by change, without the packages
// of drop, lists it with the go command and returns its packages.
func loadTree(t *testing.T, change map[string]pkg, drop ...string) []Package {
	t.Helper()
	tree := baseTree()
	maps.Copy(tree, change)
	for _, p := range drop {
		delete(tree, p)
	}
	pkgs, err := Load(t.Context(), goCommand(t), writeTree(t, tree))
	require.NoError(t, err)
	return pkgs
}

// findings returns the lines of the findings of Judge over pkgs.
func findings(pkgs []Package) []string {
	var lines []string
	for _, f := range Judge(pkgs) {
		lines = append(lines, f.String())
	}
	return lines
}

// TestImportsTable runs a fixture module for each column of the table of
// 10 10.7 and for the rule of REQ-0971, each with one import added to a
// tree that keeps every rule, and pins the findings.
func TestImportsTable(t *testing.T) {
	tests := []struct {
		name   string
		change map[string]pkg
		drop   []string
		want   []string
	}{
		{name: "the base tree keeps every rule"},
		{
			name:   "first column, a package no row names",
			change: map[string]pkg{"internal/nonesuch": {}},
			want:   []string{"internal/nonesuch: imports unnamed: no row of the table of 10 10.7 names this package; place it in a row before it builds (REQ-0972)"},
		},
		{
			name:   "first column, a subpackage follows its own row and not its parent's",
			change: map[string]pkg{"internal/cli/extra": {}},
			want:   []string{"internal/cli/extra: imports unnamed: no row of the table of 10 10.7 names this package; place it in a row before it builds (REQ-0972)"},
		},
		{
			name:   "first column, a command package follows the row of its binary",
			change: map[string]pkg{"internal/cmd/romeu/version": {imports: []string{"internal/cli", "internal/mise"}}},
			want: []string{
				"cmd/romeu: imports binary: links internal/mise, which the row of REQ-1233 does not let romeu link (10 10.7, second column)",
				"cmd/romeu: imports I1: links internal/mise, imported by internal/cmd/romeu/version: no pane or shell ever runs on the host, so romeu links no multiplexer or terminal code (I1)",
				"internal/cmd/romeu/all: imports must-not: links internal/mise, which the row of REQ-0968 must not import (10 10.7, third column)",
				"internal/cmd/romeu/version: imports link: imports internal/mise, which romeu may not link, and romeu may link this package (10 10.7, second column)",
				"internal/cmd/romeu/version: imports must-not: links internal/mise, which the row of REQ-0968 must not import (10 10.7, third column)",
			},
		},
		{
			name:   "second column, a shared package imports a package julieta alone may link",
			change: map[string]pkg{"internal/canon": {imports: []string{"internal/agent/claude"}}},
			want: []string{
				"cmd/romeu: imports binary: links internal/agent/claude, which the row of REQ-1233 does not let romeu link (10 10.7, second column)",
				"cmd/romeu: imports binary: links internal/hooks, which the row of REQ-1233 does not let romeu link (10 10.7, second column)",
				"cmd/romeu: imports I1: links internal/hooks, imported by internal/agent/claude: no pane or shell ever runs on the host, so romeu links no multiplexer or terminal code (I1)",
				"cmd/romeu: imports I2: links internal/hooks, imported by internal/agent/claude: the host never configures hooks (I2)",
				"internal/canon: imports link: imports internal/agent/claude, which romeu may not link, and romeu and julieta may link this package (10 10.7, second column)",
				"internal/cmd/romeu/all: imports must-not: links internal/hooks, which the row of REQ-0968 must not import (10 10.7, third column)",
				"internal/cmd/romeu/version: imports must-not: links internal/hooks, which the row of REQ-0968 must not import (10 10.7, third column)",
				"internal/signing: imports must-not: links internal/hooks, which the row of REQ-0968 must not import (10 10.7, third column)",
			},
		},
		{
			name:   "second column, julieta links a package of romeu",
			change: map[string]pkg{"internal/cmd/julieta/all": {imports: []string{"internal/cmd/julieta/version", "internal/signing"}}},
			want: []string{
				"cmd/julieta: imports binary: links internal/signing, which the row of REQ-0968 does not let julieta link (10 10.7, second column)",
				"internal/cmd/julieta/all: imports link: imports internal/signing, which julieta may not link, and julieta may link this package (10 10.7, second column)",
			},
		},
		{
			name:   "second column, a binary links the package no binary may link",
			change: map[string]pkg{"cmd/romeu": {imports: []string{"internal/cmd/romeu/all", "internal/cli/clidoc"}}},
			want: []string{
				"cmd/romeu: imports link: imports internal/cli/clidoc, which romeu may not link, and romeu may link this package (10 10.7, second column)",
				"cmd/romeu: imports binary: links internal/cli/clidoc, which the row of REQ-1247 does not let romeu link (10 10.7, second column)",
			},
		},
		{
			name: "third column, a package links what its row must not import, through another",
			change: map[string]pkg{
				"internal/cli/clidoc": {imports: []string{"internal/cli", "internal/mise"}},
				"internal/mise":       {imports: []string{"internal/layout"}},
			},
			want: []string{"internal/cli/clidoc: imports must-not: links internal/layout, which the row of REQ-1247 must not import (10 10.7, third column)"},
		},
		{
			name:   "tools/ci imports a command package other than through internal/cmd/<binary>/all",
			change: map[string]pkg{"tools/ci": {imports: []string{"internal/procgroup", "internal/cmd/romeu/version"}}},
			want:   []string{"tools/ci: imports tools: imports internal/cmd/romeu/version directly; tools/** and e2e/** reach internal/ through internal/cmd/<binary>/all (REQ-0971)"},
		},
		{
			name:   "a test file of tools imports a package directly",
			change: map[string]pkg{"tools/new": {imports: []string{"internal/cli"}, testImports: []string{"internal/termsafe"}}},
			want:   []string{"tools/new: imports tools: imports internal/termsafe directly; tools/** and e2e/** reach internal/ through internal/cmd/<binary>/all (REQ-0971)"},
		},
		{
			name:   "tools/new imports what only tools/ci may import",
			change: map[string]pkg{"tools/new": {imports: []string{"internal/cli/clidoc"}}},
			want:   []string{"tools/new: imports tools: imports internal/cli/clidoc directly; tools/** and e2e/** reach internal/ through internal/cmd/<binary>/all (REQ-0971)"},
		},
		{
			name:   "an e2e test imports a package directly",
			change: map[string]pkg{"e2e/x": {testImports: []string{"internal/cli"}}},
			want:   []string{"e2e/x: imports tools: imports internal/cli directly; tools/** and e2e/** reach internal/ through internal/cmd/<binary>/all (REQ-0971)"},
		},
		{
			name:   "second column, a non-test file imports a package of test packages only",
			change: map[string]pkg{"internal/cmd/julieta/version": {imports: []string{"internal/cli", "internal/cli/testsupport"}}},
			want: []string{
				"cmd/julieta: imports binary: links internal/cli/testsupport, which the row of REQ-1248 does not let julieta link (10 10.7, second column)",
				"internal/cmd/julieta/version: imports link: imports internal/cli/testsupport, which julieta may not link, and julieta may link this package (10 10.7, second column)",
				"internal/cmd/julieta/version: imports test-only: imports internal/cli/testsupport from a non-test file; only a test file imports it (REQ-1248)",
			},
		},
		{
			name: "second column, romeu links a package of julieta through a package outside internal/",
			change: map[string]pkg{
				"cmd/romeu": {imports: []string{"internal/cmd/romeu/all", "pkg/x"}},
				"pkg/x":     {imports: []string{"internal/agent/claude"}},
			},
			want: []string{
				"cmd/romeu: imports binary: links internal/agent/claude, which the row of REQ-1233 does not let romeu link (10 10.7, second column)",
				"cmd/romeu: imports binary: links internal/hooks, which the row of REQ-1233 does not let romeu link (10 10.7, second column)",
				"cmd/romeu: imports I1: links internal/hooks, imported by internal/agent/claude: no pane or shell ever runs on the host, so romeu links no multiplexer or terminal code (I1)",
				"cmd/romeu: imports I2: links internal/hooks, imported by internal/agent/claude: the host never configures hooks (I2)",
			},
		},
		{
			name: "second column, the listing holds no cmd/romeu",
			drop: []string{"cmd/romeu"},
			want: []string{"cmd/romeu: imports binary: the listing holds no such package, so its link set and the guards on it judge nothing (10 10.7)"},
		},
		{
			name: "second column, the listing holds no cmd/julieta",
			drop: []string{"cmd/julieta"},
			want: []string{"cmd/julieta: imports binary: the listing holds no such package, so its link set and the guards on it judge nothing (10 10.7)"},
		},
		{
			name: "third column, a package of julieta links sbxdrv",
			change: map[string]pkg{
				"internal/sbxdrv": {},
				"internal/mise":   {imports: []string{"internal/sbxdrv"}},
			},
			want: []string{
				"cmd/julieta: imports binary: links internal/sbxdrv, which the row of REQ-0968 does not let julieta link (10 10.7, second column)",
				"internal/cmd/julieta/all: imports must-not: links internal/sbxdrv, which the row of REQ-1233 must not import (10 10.7, third column)",
				"internal/cmd/julieta/version: imports must-not: links internal/sbxdrv, which the row of REQ-1233 must not import (10 10.7, third column)",
				"internal/mise: imports link: imports internal/sbxdrv, which julieta may not link, and julieta may link this package (10 10.7, second column)",
				"internal/mise: imports must-not: links internal/sbxdrv, which the row of REQ-1233 must not import (10 10.7, third column)",
			},
		},
		{
			name:   "third column, a shared package links layout",
			change: map[string]pkg{"internal/canon": {imports: []string{"internal/layout"}}},
			want: []string{
				"cmd/romeu: imports binary: links internal/layout, which the row of REQ-1233 does not let romeu link (10 10.7, second column)",
				"cmd/romeu: imports I1: links internal/layout, imported by internal/canon: no pane or shell ever runs on the host, so romeu links no multiplexer or terminal code (I1)",
				"internal/canon: imports link: imports internal/layout, which romeu may not link, and romeu and julieta may link this package (10 10.7, second column)",
				"internal/canon: imports must-not: links internal/layout, which the row of REQ-0969 must not import (10 10.7, third column)",
				"internal/cmd/romeu/all: imports must-not: links internal/layout, which the row of REQ-0968 must not import (10 10.7, third column)",
				"internal/cmd/romeu/version: imports must-not: links internal/layout, which the row of REQ-0968 must not import (10 10.7, third column)",
				"internal/signing: imports must-not: links internal/layout, which the row of REQ-0968 must not import (10 10.7, third column)",
			},
		},
		{
			name:   "third column, the package of test packages only links layout",
			change: map[string]pkg{"internal/cli/testsupport": {imports: []string{"internal/cli", "internal/layout"}}},
			want:   []string{"internal/cli/testsupport: imports must-not: links internal/layout, which the row of REQ-1248 must not import (10 10.7, third column)"},
		},
		{
			name:   "an e2e test of the tagged suite imports a package directly",
			change: map[string]pkg{"e2e/x": {e2eTestImports: []string{"internal/cli"}}},
			want:   []string{"e2e/x: imports tools: imports internal/cli directly; tools/** and e2e/** reach internal/ through internal/cmd/<binary>/all (REQ-0971)"},
		},
		{
			name:   "a package imports encoding/json v1",
			change: map[string]pkg{"internal/canon": {imports: []string{"encoding/json"}}},
			want:   []string{"internal/canon: imports json-v1: imports encoding/json; JSON is encoding/json/v2 (REQ-0960)"},
		},
		{
			name:   "a test file imports encoding/json v1",
			change: map[string]pkg{"internal/canon": {testImports: []string{"encoding/json"}}},
			want:   []string{"internal/canon: imports json-v1: imports encoding/json; JSON is encoding/json/v2 (REQ-0960)"},
		},
		{
			name:   "an e2e test of the tagged suite imports encoding/json v1",
			change: map[string]pkg{"internal/canon": {e2eTestImports: []string{"encoding/json"}}},
			want:   []string{"internal/canon: imports json-v1: imports encoding/json; JSON is encoding/json/v2 (REQ-0960)"},
		},
		{
			name:   "encoding/json/v2 and encoding/json/jsontext are no finding",
			change: map[string]pkg{"internal/canon": {imports: []string{"encoding/json/v2", "encoding/json/jsontext"}, testImports: []string{"encoding/json/v2"}}},
		},
		{
			name:   "internal/render links text/template",
			change: map[string]pkg{"internal/render": {imports: []string{"text/template"}}},
			want:   []string{"internal/render: imports I11: links text/template: YAML and JSON are written only through marshallers (I11)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, findings(loadTree(t, tt.change, tt.drop...)))
		})
	}
}

// linksOf returns the packages cmd/romeu links in pkgs: the positive
// control of the guards below, which shows that the plant reached the
// link set and so that the guard, not an inert plant, makes the test
// pass.
func linksOf(t *testing.T, pkgs []Package) []string {
	t.Helper()
	i := slices.IndexFunc(pkgs, func(p Package) bool { return p.Path == "cmd/romeu" })
	require.GreaterOrEqual(t, i, 0, "the fixture holds cmd/romeu")
	return pkgs[i].Deps
}

// TestImportsHostLinksNoTerminalCode is the one-violation fixture of I1:
// cmd/romeu imports internal/layout, and Judge reports it, the finding
// of the guard among them.
//
//romeu:invariant I1 test
func TestImportsHostLinksNoTerminalCode(t *testing.T) {
	pkgs := loadTree(t, map[string]pkg{"cmd/romeu": {imports: []string{"internal/cmd/romeu/all", "internal/layout"}}})
	require.Contains(t, linksOf(t, pkgs), "internal/layout", "the plant reached the link set")
	assert.Equal(t, []string{
		"cmd/romeu: imports link: imports internal/layout, which romeu may not link, and romeu may link this package (10 10.7, second column)",
		"cmd/romeu: imports binary: links internal/layout, which the row of REQ-1233 does not let romeu link (10 10.7, second column)",
		"cmd/romeu: imports I1: links internal/layout, imported by cmd/romeu: no pane or shell ever runs on the host, so romeu links no multiplexer or terminal code (I1)",
	}, findings(pkgs))
}

// TestImportsHostConfiguresNoHooks is the one-violation fixture of I2:
// the command list of romeu imports internal/hooks, and Judge reports
// it, the finding of the guard among them.
//
//romeu:invariant I2 test
func TestImportsHostConfiguresNoHooks(t *testing.T) {
	pkgs := loadTree(t, map[string]pkg{"internal/cmd/romeu/all": {imports: []string{"internal/cmd/romeu/version", "internal/hooks"}}})
	require.Contains(t, linksOf(t, pkgs), "internal/hooks", "the plant reached the link set")
	assert.Equal(t, []string{
		"cmd/romeu: imports binary: links internal/hooks, which the row of REQ-1233 does not let romeu link (10 10.7, second column)",
		"cmd/romeu: imports I1: links internal/hooks, imported by internal/cmd/romeu/all: no pane or shell ever runs on the host, so romeu links no multiplexer or terminal code (I1)",
		"cmd/romeu: imports I2: links internal/hooks, imported by internal/cmd/romeu/all: the host never configures hooks (I2)",
		"internal/cmd/romeu/all: imports link: imports internal/hooks, which romeu may not link, and romeu may link this package (10 10.7, second column)",
		"internal/cmd/romeu/all: imports must-not: links internal/hooks, which the row of REQ-0968 must not import (10 10.7, third column)",
	}, findings(pkgs))
}

// TestImportsHostLinksNoMemoryWriter is the one-violation fixture of the
// I24 rule of 10 10.7: a package romeu links imports
// internal/memstore/write, and the guard reports it.
//
//romeu:invariant I24 test
func TestImportsHostLinksNoMemoryWriter(t *testing.T) {
	pkgs := loadTree(t, map[string]pkg{"internal/signing": {imports: []string{"internal/memstore/write"}}})
	require.Contains(t, linksOf(t, pkgs), "internal/memstore/write", "the plant reached the link set")
	assert.Equal(t, []string{
		"cmd/romeu: imports binary: links internal/memstore/write, which the row of REQ-1233 does not let romeu link (10 10.7, second column)",
		"cmd/romeu: imports I24: links internal/memstore/write, imported by internal/signing: romeu reads the memory dirs only through the readers of internal/memstore (I24)",
		"internal/signing: imports link: imports internal/memstore/write, which romeu may not link, and romeu may link this package (10 10.7, second column)",
	}, findings(pkgs))
}

// TestImportsLoadUnitesTheTargets shows that a file that builds on one
// target alone is judged: an import in a darwin file of cmd/julieta
// fails on every machine.
func TestImportsLoadUnitesTheTargets(t *testing.T) {
	dir := writeTree(t, baseTree())
	src := "package main\n\n" + importLine("internal/signing")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cmd", "julieta", "p_darwin.go"), []byte(src), 0o600))
	pkgs, err := Load(t.Context(), goCommand(t), dir)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"cmd/julieta: imports link: imports internal/signing, which julieta may not link, and julieta may link this package (10 10.7, second column)",
		"cmd/julieta: imports binary: links internal/signing, which the row of REQ-0968 does not let julieta link (10 10.7, second column)",
	}, findings(pkgs))
}

// TestImportsLoadFailsOnAPackageThatDoesNotLoad shows that a package go
// list cannot load is an error, not a pass.
func TestImportsLoadFailsOnAPackageThatDoesNotLoad(t *testing.T) {
	dir := writeTree(t, map[string]pkg{"internal/cli": {imports: []string{"internal/nonesuch"}}})
	_, err := Load(t.Context(), goCommand(t), dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "go list for linux/amd64: "+citest.FixtureModule+"/internal/cli: ")
}

// TestImportsReport pins the output of the subcommand over a fixture
// with no finding, over one without internal/render, which says that
// the I11 rule judged nothing, and over one with a finding.
func TestImportsReport(t *testing.T) {
	var out bytes.Buffer
	ok, err := report(t.Context(), &out, goCommand(t), writeTree(t, baseTree()))
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "imports: ok\n", out.String())

	tree := baseTree()
	delete(tree, "internal/render")
	out.Reset()
	ok, err = report(t.Context(), &out, goCommand(t), writeTree(t, tree))
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "imports: internal/render does not exist yet, so the I11 rule judged nothing\nimports: ok\n", out.String())

	tree = baseTree()
	tree["internal/nonesuch"] = pkg{}
	out.Reset()
	ok, err = report(t.Context(), &out, goCommand(t), writeTree(t, tree))
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "internal/nonesuch: imports unnamed: no row of the table of 10 10.7 names this package; place it in a row before it builds (REQ-0972)\nimports: 1 finding(s)\n", out.String())
}

// TestImportsRunRefusesAnArgument checks the usage of the subcommand.
func TestImportsRunRefusesAnArgument(t *testing.T) {
	var out bytes.Buffer
	ok, err := Run(t.Context(), check.Env{Dir: t.TempDir(), Stdout: &out, Stderr: &out}, []string{"x"})
	require.Error(t, err)
	assert.False(t, ok)
	assert.Contains(t, err.Error(), "imports takes no argument")
}

// TestImportsSteps pins the step of all: the subcommand, run with the
// pinned go in the environment of the steps.
func TestImportsSteps(t *testing.T) {
	assert.Equal(t, []check.Step{{
		Name:    "imports",
		Argv:    []string{"/pinned/bin/go", "run", "./tools/ci", "imports"},
		Environ: check.StepEnv("/pinned/bin"),
	}}, Steps(t.Context(), check.Plan{Root: "/src", Tools: check.Tools{GoDir: "/pinned/bin"}}))
}

// TestImportsRowOf pins how a path below internal/ finds its row.
func TestImportsRowOf(t *testing.T) {
	for name, req := range map[string]string{
		"signing":           "REQ-0968",
		"cmd/romeu/all":     "REQ-0968",
		"cmd/romeu/version": "REQ-0968",
		"cmd/julieta/all":   "REQ-1233",
		"agent/claude":      "REQ-1233",
		"memstore/write":    "REQ-1233",
		"memstore":          "REQ-0969",
		"procgroup":         "REQ-0969",
		"cli/clidoc":        "REQ-1247",
		"cli/testsupport":   "REQ-1248",
	} {
		row, ok := RowOf(name)
		if assert.True(t, ok, name) {
			assert.Equal(t, req, row.Req, name)
		}
	}
	for _, name := range []string{"agent", "agent/claude/x", "cmd", "cmd/romeu", "cmd/other/x", "cli/testsupport/x", "nonesuch"} {
		_, ok := RowOf(name)
		assert.False(t, ok, name)
	}
}

// TestImportsRowsAreTheTableOf107 holds Rows to the table of 10 10.7 in
// the specification: each row's id, the packages of its first column,
// the binaries of its second and the packages of its third, in order.
func TestImportsRowsAreTheTableOf107(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), "docs", "spec", "10-testing-style.md"))
	require.NoError(t, err)
	_, rest, found := strings.Cut(string(b), "\n| Package | May be imported by | Must not import |\n|---|---|---|\n")
	require.True(t, found, "the header of the table of 10 10.7")
	binaries := map[string]Binaries{
		"`cmd/romeu`, `tools/ci`": Romeu,
		"`cmd/julieta` only":      Julieta,
		"both":                    Romeu | Julieta,
		"`tools/ci` only":         0,
		"test packages only":      0,
	}
	span := regexp.MustCompile("`([^`]+)`")
	spans := func(cell string) []string {
		var out []string
		for _, m := range span.FindAllStringSubmatch(cell, -1) {
			out = append(out, m[1])
		}
		return out
	}
	var got []Row
	for line := range strings.SplitSeq(rest, "\n") {
		if !strings.HasPrefix(line, "| ") {
			break
		}
		cells := strings.Split(strings.Trim(line, "| "), " | ")
		require.Len(t, cells, 3, line)
		first := spans(cells[0])
		require.NotEmpty(t, first, line)
		linked, ok := binaries[cells[1]]
		require.True(t, ok, "the second column of %s", line)
		got = append(got, Row{Req: first[0], Names: first[1:], LinkedBy: linked, MustNot: spans(cells[2]),
			TestOnly: cells[1] == "test packages only"})
	}
	assert.Equal(t, Rows, got)
}
