// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// forbiddenTestImports are the packages of testify that ADR 0007, rule
// 1, leaves out of the tests.
var forbiddenTestImports = []string{
	"github.com/stretchr/testify/suite",
	"github.com/stretchr/testify/mock",
	"github.com/stretchr/testify/http",
}

// forbiddenTestCalls are the functions a test does not name, by the
// import path of their package: assert.New and require.New (ADR 0007,
// rule 1) and reflect.DeepEqual (rule 5).
var forbiddenTestCalls = map[string][]string{
	"github.com/stretchr/testify/assert":  {"New"},
	"github.com/stretchr/testify/require": {"New"},
	"reflect":                             {"DeepEqual"},
}

// testStyleFindings returns what the Go file holds against rules 1 and
// 5 of ADR 0007, when it is test code: a file named *_test.go, or a
// file of a package whose name ends in "test", as gittest. It reports
// an import of forbiddenTestImports, a reference to one of
// forbiddenTestCalls through its package, and a dot import of one of
// those packages, through which a bare name could not be told apart.
func testStyleFindings(name string, src []byte) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), name, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(name, "_test.go") && !strings.HasSuffix(f.Name.Name, "test") {
		return nil, nil
	}
	var bad []string
	byName := map[string]string{} // the name a file gives a package -> its import path
	for _, imp := range f.Imports {
		importPath, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, err
		}
		if slices.Contains(forbiddenTestImports, importPath) {
			bad = append(bad, "import "+importPath)
		}
		if _, ok := forbiddenTestCalls[importPath]; !ok {
			continue
		}
		local := path.Base(importPath)
		if imp.Name != nil {
			local = imp.Name.Name
		}
		if local == "." {
			bad = append(bad, "dot import "+importPath)
		}
		byName[local] = importPath
	}
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); ok {
			if importPath, ok := byName[x.Name]; ok && slices.Contains(forbiddenTestCalls[importPath], sel.Sel.Name) {
				bad = append(bad, path.Base(importPath)+"."+sel.Sel.Name)
			}
		}
		return true
	})
	return bad, nil
}

// TestTestStyleFindings pins testStyleFindings on each form it reports
// and on the files it leaves alone.
func TestTestStyleFindings(t *testing.T) {
	const assertPkg, requirePkg = `"github.com/stretchr/testify/assert"`, `"github.com/stretchr/testify/require"`
	tests := []struct {
		name string
		file string
		src  string
		want []string
	}{
		{"package functions", "a_test.go", "package p\n\nimport " + assertPkg + "\n\nvar _ = assert.Equal\n", nil},
		{"assert.New", "a_test.go", "package p\n\nimport " + assertPkg + "\n\nvar _ = assert.New(nil)\n", []string{"assert.New"}},
		{"require.New under another name", "a_test.go", "package p\n\nimport r " + requirePkg + "\n\nvar _ = r.New\n", []string{"require.New"}},
		{"suite", "a_test.go", "package p\n\nimport _ \"github.com/stretchr/testify/suite\"\n", []string{"import github.com/stretchr/testify/suite"}},
		{"mock", "a_test.go", "package p\n\nimport _ \"github.com/stretchr/testify/mock\"\n", []string{"import github.com/stretchr/testify/mock"}},
		{"http", "a_test.go", "package p\n\nimport _ \"github.com/stretchr/testify/http\"\n", []string{"import github.com/stretchr/testify/http"}},
		{"reflect.DeepEqual", "a_test.go", "package p\n\nimport \"reflect\"\n\nvar _ = reflect.DeepEqual(1, 1)\n", []string{"reflect.DeepEqual"}},
		{"a dot import of reflect", "a_test.go", "package p\n\nimport . \"reflect\"\n\nvar _ = DeepEqual(1, 1)\n", []string{"dot import reflect"}},
		{"another function of reflect", "a_test.go", "package p\n\nimport \"reflect\"\n\nvar _ = reflect.TypeOf(1)\n", nil},
		{"a test helper package", "fixture.go", "package fixturetest\n\nimport \"reflect\"\n\nvar _ = reflect.DeepEqual(1, 1)\n", []string{"reflect.DeepEqual"}},
		{"code that is not a test", "a.go", "package p\n\nimport \"reflect\"\n\nvar _ = reflect.DeepEqual(1, 1)\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := testStyleFindings(tt.file, []byte(tt.src))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestTestStyle checks rules 1 and 5 of ADR 0007 over the Go files of
// the repository, tracked and not yet added: no test imports suite,
// mock or http of testify, or names assert.New, require.New or
// reflect.DeepEqual.
func TestTestStyle(t *testing.T) {
	root := citest.ModuleRoot(t)
	files := repoFiles(t, root, "*.go")
	require.NotEmpty(t, files, "the Go files of the repository")
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(root, f))
		require.NoError(t, err)
		bad, err := testStyleFindings(f, data)
		require.NoError(t, err, "parse %s", f)
		assert.Empty(t, bad, "%s holds what ADR 0007 keeps out of tests", f)
	}
}
