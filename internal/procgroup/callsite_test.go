// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// thisPackage is the path of this package below the module root.
const thisPackage = "internal/procgroup"

// admittedExec are the directories, below the module root, whose files
// may import os/exec: this package, and, until T105 switches them to
// it, tools/ci and tools/mutants, whose own copy of the group rule
// lives in tools/ci/procgroup. T105 removes the two tools entries.
var admittedExec = []string{thisPackage, "tools/ci", "tools/mutants"}

// TestCallSites is the call-site scan of 04 4.1, Subprocesses, item 4,
// beside the I16 scan of 05 5.2, over this module: no file outside the
// admitted directories imports os/exec, so every subprocess starts
// here, and only a test file sets the seam of the timeout table.
func TestCallSites(t *testing.T) {
	root := moduleRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	require.NoError(t, err)
	module := modulePath(data)
	require.NotEmpty(t, module, "the module line of go.mod")
	findings, err := callSiteFindings(root, module, admittedExec)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

// callSiteFixture is the tree of the positive control, written to a
// temporary directory, since a Go file under testdata would escape the
// linters: a package outside procgroup that calls exec.Command, in a
// source file and in a test file, and that sets the seam outside a
// test; and what the scan admits or leaves out.
var callSiteFixture = map[string]string{
	"internal/procgroup/run.go": `package procgroup

import "os/exec"

func Start(name string) error { return exec.Command(name).Start() }

func WithTestTimeout() {}
`,
	"internal/fixture/exec.go": `package fixture

import "os/exec"

func Start() error { return exec.Command("gh").Start() }
`,
	"internal/fixture/seam.go": `package fixture

import pg "example.com/fixture/internal/procgroup"

func Shorten() { pg.WithTestTimeout() }
`,
	"internal/fixture/seam_test.go": `package fixture

import (
	"os/exec"
	"testing"

	"example.com/fixture/internal/procgroup"
)

func TestShorten(t *testing.T) {
	procgroup.WithTestTimeout()
	_ = exec.Command("true").Run()
}
`,
	"internal/fixture/other.go": `package fixture

import "example.com/fixture/internal/other"

func Other() { other.WithTestTimeout() }
`,
	"tools/ci/ci.go": `package ci

import "os/exec"

func Start(name string) error { return exec.Command(name).Start() }
`,
	"internal/fixture/testdata/ignored.go": `package ignored

import "os/exec"

func Start(name string) error { return exec.Command(name).Start() }
`,
	"internal/.hidden/ignored.go": `package ignored

import "os/exec"
`,
}

// TestCallSiteFindings is the positive control of the scan, over
// callSiteFixture.
func TestCallSiteFindings(t *testing.T) {
	root := t.TempDir()
	for name, content := range callSiteFixture {
		p := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	}
	findings, err := callSiteFindings(root, "example.com/fixture", admittedExec)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"internal/fixture/exec.go: imports os/exec outside internal/procgroup",
		"internal/fixture/seam.go: sets procgroup.WithTestTimeout outside a test file",
		"internal/fixture/seam_test.go: imports os/exec outside internal/procgroup",
	}, findings)
}

// moduleRoot returns the directory of the go.mod above this package.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "a go.mod above %s", dir)
		dir = parent
	}
}

// modulePath returns the path of the module line of a go.mod, or ""
// when it has none. A quoted path is not used in this repository.
func modulePath(gomod []byte) string {
	for line := range strings.Lines(string(gomod)) {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// callSiteFindings walks the Go files of the tree at root, a module
// whose path is module, and returns one line per breach, sorted: a file
// outside the admitted directories that imports os/exec, and a file
// other than a test file that refers to WithTestTimeout of this
// package. It leaves out what the go command leaves out: testdata, and
// directories whose name starts with "." or "_".
func callSiteFindings(root, module string, admitted []string) ([]string, error) {
	self := module + "/" + thisPackage
	var findings []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		file, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		findings = append(findings, fileFindings(rel, file, self, admitted)...)
		return nil
	})
	slices.Sort(findings)
	return findings, err
}

// fileFindings returns the breaches of one file, whose path below the
// module root is rel.
func fileFindings(rel string, file *ast.File, self string, admitted []string) []string {
	var findings []string
	seam := map[string]bool{} // the names this file gives this package
	for _, spec := range file.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		switch imported {
		case "os/exec":
			if !inside(rel, admitted) {
				findings = append(findings, rel+": imports os/exec outside "+thisPackage)
			}
		case self:
			name := path.Base(self)
			if spec.Name != nil {
				name = spec.Name.Name
			}
			seam[name] = true
		}
	}
	if strings.HasSuffix(rel, "_test.go") || len(seam) == 0 {
		return findings
	}
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "WithTestTimeout" {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); ok && seam[x.Name] {
			findings = append(findings, rel+": sets procgroup.WithTestTimeout outside a test file")
		}
		return true
	})
	return findings
}

// inside reports whether the slash path rel is in one of the dirs.
func inside(rel string, dirs []string) bool {
	return slices.ContainsFunc(dirs, func(dir string) bool { return strings.HasPrefix(rel, dir+"/") })
}
