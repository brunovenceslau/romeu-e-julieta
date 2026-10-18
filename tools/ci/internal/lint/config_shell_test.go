// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// shellRoots are the directories of the product, where REQ-0959 holds:
// no "sh -c" anywhere in romeu. Test files are left out, because the
// tests of the subprocess package start "sh" as a program to start, and
// tools/ is the build of the repository, which does the same.
var shellRoots = []string{"cmd", "internal"}

// shellCommands are the program names that start a shell line.
var shellCommands = []string{"sh", "/bin/sh"}

// shellFindings returns the places in src, a Go file, where a call or a
// composite literal holds "sh" and then "-c" as consecutive string
// literals, the way an argv or an exec call spells a shell command. It
// is the check that forbidigo cannot make: forbidigo reads identifiers
// and selectors, never a string argument.
func shellFindings(name string, src []byte) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, err
	}
	var found []string
	check := func(pos token.Pos, args []ast.Expr) {
		for i := 0; i+1 < len(args); i++ {
			if literal(args[i], shellCommands...) && literal(args[i+1], "-c") {
				found = append(found, fset.Position(pos).String()+": a shell command, \"sh\" with \"-c\" (REQ-0959)")
				return
			}
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			check(x.Pos(), x.Args)
		case *ast.CompositeLit:
			check(x.Pos(), x.Elts)
		}
		return true
	})
	return found, nil
}

// literal says whether e is a string literal equal to one of values.
func literal(e ast.Expr, values ...string) bool {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	s, err := strconv.Unquote(lit.Value)
	return err == nil && slices.Contains(values, s)
}

// TestShellFindings is the fixture of the check: one violation of each
// shape, and the calls that are none.
func TestShellFindings(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want int
	}{
		{"a call", `package x
func f() { run("sh", "-c", "echo") }
`, 1},
		{"an argv", `package x
var argv = []string{"sh", "-c", "echo"}
`, 1},
		{"the full path", `package x
func f() { run("/bin/sh", "-c", "echo") }
`, 1},
		{"sh without -c", `package x
func f() { run("sh", "script.sh") }
`, 0},
		{"-c before sh", `package x
func f() { run("-c", "sh") }
`, 0},
		{"a variable", `package x
func f(c string) { run("sh", c) }
`, 0},
		{"a string that mentions it", `package x
const s = "sh -c"
`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := shellFindings("x.go", []byte(tt.src))
			require.NoError(t, err)
			assert.Len(t, got, tt.want)
		})
	}
}

// TestNoShellCommands holds REQ-0959 for the product: no Go file of cmd
// or internal, tests apart, starts a program through "sh -c".
func TestNoShellCommands(t *testing.T) {
	root := citest.ModuleRoot(t)
	files := 0
	for _, dir := range shellRoots {
		require.DirExists(t, filepath.Join(root, dir))
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && d.Name() == "testdata" {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			files++
			found, err := shellFindings(p, src)
			if err != nil {
				return err
			}
			assert.Empty(t, found)
			return nil
		})
		require.NoError(t, err)
	}
	assert.Positive(t, files, "the walk reads Go files")
}
