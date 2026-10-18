// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// reviveGenerated reports whether revive v1.17.0 skips the Go source
// as generated: a line that starts with "// Code generated " and ends
// with " DO NOT EDIT." (isGenerated in lint/linter.go). It is built in
// parts so that this file is not one.
func reviveGenerated(src []byte) bool {
	prefix, suffix := "// Code gener"+"ated ", " DO NOT ED"+"IT."
	for line := range strings.SplitSeq(string(src), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, prefix) && strings.HasSuffix(line, suffix) && len(line) >= len(prefix)+len(suffix) {
			return true
		}
	}
	return false
}

// reviveRuleProblems returns what revive's two rules of ADR 0001, rule
// 14, look at in the Go source: each exported identifier, which needs a
// doc comment, and each call of errors.New or fmt.Errorf, whose string
// follows Go style, under whatever name the file imports the package;
// and a dot import, which would hide such a call. revive skips a
// generated file (ADR 0016), so a generated file holds none of them.
func reviveRuleProblems(src []byte) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var problems []string
	// The local name of each package whose constructor revive's
	// error-strings rule reads, as the file imports it.
	errorPackages := map[string]string{}
	for _, spec := range f.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		name := path[strings.LastIndex(path, "/")+1:]
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "." {
			problems = append(problems, "a dot import of "+path)
		}
		if path == "errors" || path == "fmt" {
			errorPackages[name] = path
		}
	}
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.IsExported() {
				problems = append(problems, "exported "+d.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.IsExported() {
						problems = append(problems, "exported "+s.Name.Name)
					}
				case *ast.ValueSpec:
					for _, n := range s.Names {
						if n.IsExported() {
							problems = append(problems, "exported "+n.Name)
						}
					}
				}
			}
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			// a.b.C() and f().M() have no package name before the dot.
			if x, ok := sel.X.(*ast.Ident); ok {
				if path := errorPackages[x.Name]; (path == "errors" && sel.Sel.Name == "New") || (path == "fmt" && sel.Sel.Name == "Errorf") {
					problems = append(problems, "an error string ("+path+"."+sel.Sel.Name+")")
				}
			}
		}
		return true
	})
	return problems, nil
}

// TestReviveRuleProblems pins reviveGenerated and reviveRuleProblems.
func TestReviveRuleProblems(t *testing.T) {
	assert.True(t, reviveGenerated([]byte("// SPDX\n\n"+generatedHeader+"package p\n")))
	assert.True(t, reviveGenerated([]byte(strings.ReplaceAll(generatedHeader, "\n", "\r\n")+"package p\n")))
	assert.False(t, reviveGenerated([]byte("package p\n\n// x // Code gener"+"ated by y. DO NOT ED"+"IT.\n")))
	got, err := reviveRuleProblems([]byte(`package p

import (
	"errors"
	"fmt"
)

const unexported = 1

// Doc is documented, and still exported.
var Doc, other = 1, 2

type T struct{}

func (T) M() {}

func F() error { return errors.New("x") }

func g() error { return fmt.Errorf("y") }
`))
	require.NoError(t, err)
	assert.Equal(t, []string{"exported Doc", "exported T", "exported M", "exported F", "an error string (errors.New)", "an error string (fmt.Errorf)"}, got)
	got, err = reviveRuleProblems([]byte("package p\n\nimport (\n\te \"errors\"\n\t. \"fmt\"\n)\n\nvar _ = e.New(\"x\")\nvar _ = Sprint\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{"a dot import of fmt", "an error string (errors.New)"}, got, "an alias and a dot import")
	got, err = reviveRuleProblems([]byte("package p\n\nfunc g() {\n\ta.b.C()\n\tf().M()\n}\n"))
	require.NoError(t, err)
	assert.Empty(t, got, "a call on a selector or a call result")
}

// TestGeneratedGoFilesKeepReviveRules holds the Go files of the
// repository that revive skips as generated: each is an output of go
// generate, listed with -text in the root .gitattributes, which the
// generated check holds to the outputs, so a hand-written file cannot
// carry the marker; and each holds nothing revive's rules would find:
// no exported identifier, no error string and no dot import (ADR 0016).
func TestGeneratedGoFilesKeepReviveRules(t *testing.T) {
	root := citest.ModuleRoot(t)
	attrs, err := os.ReadFile(filepath.Join(root, ".gitattributes"))
	require.NoError(t, err)
	listed := map[string]bool{}
	for line := range strings.SplitSeq(string(attrs), "\n") {
		if path, ok := strings.CutSuffix(strings.TrimSpace(line), " -text"); ok {
			listed[path] = true
		}
	}
	var generated []string
	for _, f := range repoFiles(t, root, "*.go") {
		data, err := os.ReadFile(filepath.Join(root, f))
		require.NoError(t, err)
		if !reviveGenerated(data) {
			continue
		}
		generated = append(generated, filepath.ToSlash(f))
		assert.True(t, listed[filepath.ToSlash(f)], "%s has the generated header but is no output of go generate listed in .gitattributes (ADR 0016)", f)
		problems, err := reviveRuleProblems(data)
		require.NoError(t, err, f)
		assert.Empty(t, problems, "%s has the generated header, which revive skips, so it holds no exported identifier, no error string and no dot import (ADR 0016)", f)
	}
	assert.Contains(t, generated, "internal/cli/exitcodes_gen.go", "the scan saw the generated Go files")
}
