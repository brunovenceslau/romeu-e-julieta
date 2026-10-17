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

// skipCalls are the methods of testing.T, B and F that skip a test (10
// 10.6, REQ-0964). A test that cannot run fails instead.
var skipCalls = []string{"Skip", "SkipNow", "Skipf"}

// skipFindings returns what the Go file holds against the skip and
// TestMain rules of 10 10.6: in any file that imports testing (a test,
// or a helper package of tests, whatever its name), a reference to a
// method named Skip, SkipNow or Skipf that is called, or whose receiver
// is a variable of a testing type or that type itself, so a method
// value passed to a helper is caught and a field named Skip is not, and
// a reference to testing.Short; in a *_test.go file, a TestMain whose
// exit status is not the one m.Run returned. The TestMain rule reads
// names, not flow: a holder of m.Run's result reassigned before
// os.Exit, or an exit through syscall or log.Fatal, passes it. A TestMain passes in
// either spelling: os.Exit(m.Run()), or the result of m.Run stored in a
// variable that every os.Exit of the function exits with; one that
// never calls os.Exit returns m.Run's status itself.
func skipFindings(name string, src []byte) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	testingName, osName := "", ""
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, err
		}
		local := path.Base(p)
		if imp.Name != nil {
			local = imp.Name.Name
		}
		switch p {
		case "testing":
			testingName = local
		case "os":
			osName = local
		}
	}
	if testingName == "" {
		return nil, nil
	}
	isTestFile := strings.HasSuffix(name, "_test.go")
	testVars := testingVars(f, testingName)
	called := map[*ast.SelectorExpr]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				called[sel] = true
			}
		}
		return true
	})
	var bad []string
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			if slices.Contains(skipCalls, n.Sel.Name) && (called[n] || isTestingValue(n.X, testVars, testingName)) {
				bad = append(bad, fset.Position(n.Pos()).String()+": "+n.Sel.Name)
			}
			if x, ok := n.X.(*ast.Ident); ok && x.Name == testingName && n.Sel.Name == "Short" {
				bad = append(bad, fset.Position(n.Pos()).String()+": testing.Short")
			}
		case *ast.FuncDecl:
			if isTestFile && n.Recv == nil && n.Name.Name == "TestMain" && n.Body != nil {
				bad = append(bad, testMainFindings(fset, n, osName)...)
			}
		}
		return true
	})
	return bad, nil
}

// testingTypes are the types of testing whose methods skip a test.
var testingTypes = []string{"T", "B", "F", "TB"}

// isTestingType reports whether e is one of testingTypes, or a pointer
// to one, named through the file's name for testing.
func isTestingType(e ast.Expr, testingName string) bool {
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == testingName && slices.Contains(testingTypes, sel.Sel.Name)
}

// testingVars returns the names the file declares as parameters or
// variables of a type of testingTypes.
func testingVars(f *ast.File, testingName string) map[string]bool {
	vars := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Field:
			if isTestingType(n.Type, testingName) {
				for _, id := range n.Names {
					vars[id.Name] = true
				}
			}
		case *ast.ValueSpec:
			if n.Type != nil && isTestingType(n.Type, testingName) {
				for _, id := range n.Names {
					vars[id.Name] = true
				}
			}
		}
		return true
	})
	return vars
}

// isTestingValue reports whether x, the receiver of a selector, is a
// variable of a testing type, or the type itself as in the method
// expression (*testing.T).Skip.
func isTestingValue(x ast.Expr, vars map[string]bool, testingName string) bool {
	if paren, ok := x.(*ast.ParenExpr); ok {
		x = paren.X
	}
	if id, ok := x.(*ast.Ident); ok {
		return vars[id.Name]
	}
	return isTestingType(x, testingName)
}

// testMainFindings returns why fn, a TestMain, may exit with another
// status than the one m.Run returned: it calls no m.Run, or an os.Exit
// of it exits with something other than m.Run() or a variable that
// holds its result.
func testMainFindings(fset *token.FileSet, fn *ast.FuncDecl, osName string) []string {
	m := ""
	if params := fn.Type.Params.List; len(params) == 1 && len(params[0].Names) == 1 {
		m = params[0].Names[0].Name
	}
	isRun := func(e ast.Expr) bool {
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		x, ok := sel.X.(*ast.Ident)
		return ok && m != "" && x.Name == m && sel.Sel.Name == "Run"
	}
	runs := false
	holders := map[string]bool{} // the variables m.Run's result is stored in
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok && len(as.Lhs) == 1 && len(as.Rhs) == 1 && isRun(as.Rhs[0]) {
			if id, ok := as.Lhs[0].(*ast.Ident); ok {
				holders[id.Name] = true
			}
		}
		if vs, ok := n.(*ast.ValueSpec); ok && len(vs.Names) == 1 && len(vs.Values) == 1 && isRun(vs.Values[0]) {
			holders[vs.Names[0].Name] = true
		}
		if e, ok := n.(ast.Expr); ok && isRun(e) {
			runs = true
		}
		return true
	})
	pos := fset.Position(fn.Pos()).String()
	if !runs {
		return []string{pos + ": TestMain calls no m.Run"}
	}
	var bad []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Exit" {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); !ok || osName == "" || x.Name != osName {
			return true
		}
		if len(call.Args) == 1 {
			if isRun(call.Args[0]) {
				return true
			}
			if id, ok := call.Args[0].(*ast.Ident); ok && holders[id.Name] {
				return true
			}
		}
		bad = append(bad, fset.Position(call.Pos()).String()+": TestMain exits with another status than m.Run's")
		return true
	})
	return bad
}

// TestTestStyleSkipFindings pins skipFindings on each form it reports
// and on the files it leaves alone: the skips, testing.Short, and the
// TestMain rule in both spellings.
func TestTestStyleSkipFindings(t *testing.T) {
	const head = "package p\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\n"
	tests := []struct {
		name string
		file string
		src  string
		want []string
	}{
		{"t.Skip", "a_test.go", head + "func TestA(t *testing.T) { t.Skip(\"x\") }\nvar _ = os.Exit\n", []string{"a_test.go:8:28: Skip"}},
		{"t.SkipNow", "a_test.go", head + "func TestA(t *testing.T) { t.SkipNow() }\nvar _ = os.Exit\n", []string{"a_test.go:8:28: SkipNow"}},
		{"b.Skipf", "a_test.go", head + "func BenchmarkA(b *testing.B) { b.Skipf(\"x\") }\nvar _ = os.Exit\n", []string{"a_test.go:8:33: Skipf"}},
		{"testing.Short", "a_test.go", head + "var _ = testing.Short\nvar _ = os.Exit\n", []string{"a_test.go:8:9: testing.Short"}},
		{"os.Exit(m.Run())", "a_test.go", head + "func TestMain(m *testing.M) { os.Exit(m.Run()) }\n", nil},
		{"the result stored, cleaned up and exited", "a_test.go", head + "func TestMain(m *testing.M) {\n\tcode := m.Run()\n\t_ = os.RemoveAll(\"x\")\n\tos.Exit(code)\n}\n", nil},
		{"the result stored with var", "a_test.go", head + "func TestMain(m *testing.M) {\n\tvar code = m.Run()\n\tos.Exit(code)\n}\n", nil},
		{"a field named Skip", "a_test.go", head + "type step struct{ Skip string }\nvar _ = step{}.Skip\nvar _ = os.Exit\n", nil},
		{"a TestMain that returns", "a_test.go", head + "func TestMain(m *testing.M) { m.Run() }\nvar _ = os.Exit\n", nil},
		{"exit 0 whatever m.Run returned", "a_test.go", head + "func TestMain(m *testing.M) {\n\tm.Run()\n\tos.Exit(0)\n}\n", []string{"a_test.go:10:2: TestMain exits with another status than m.Run's"}},
		{"no m.Run", "a_test.go", head + "func TestMain(m *testing.M) { os.Exit(0) }\n", []string{"a_test.go:8:1: TestMain calls no m.Run"}},
		{"a method value passed on", "a_test.go", head + "func TestA(t *testing.T) { s := t.Skip; s(\"x\") }\nvar _ = os.Exit\n", []string{"a_test.go:8:33: Skip"}},
		{"a method expression", "a_test.go", head + "var _ = (*testing.T).SkipNow\nvar _ = os.Exit\n", []string{"a_test.go:8:9: SkipNow"}},
		{"a helper package of any name", "fixtures/fixtures.go", head + "func Skip(t testing.TB) { t.Skip() }\nvar _ = os.Exit\n", []string{"fixtures/fixtures.go:8:27: Skip"}},
		{"a TestMain outside a test file", "a.go", head + "func TestMain(m *testing.M) { os.Exit(0) }\n", nil},
		{"a file that does not import testing", "a.go", "package p\n\ntype s struct{}\n\nfunc (s) Skip() {}\n\nvar _ = s{}.Skip\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := skipFindings(tt.file, []byte(tt.src))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestTestStyleSkips checks the skip and TestMain rules of 10 10.6 over
// the Go files of the repository, tracked and not yet added.
func TestTestStyleSkips(t *testing.T) {
	root := citest.ModuleRoot(t)
	files := repoFiles(t, root, "*.go")
	require.NotEmpty(t, files, "the Go files of the repository")
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(root, f))
		require.NoError(t, err)
		bad, err := skipFindings(f, data)
		require.NoError(t, err, "parse %s", f)
		assert.Empty(t, bad, "%s skips a test or exits TestMain with another status (10 10.6)", f)
	}
}
