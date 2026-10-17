// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli_test

import (
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// root is the repository, from this package's directory.
const root = "../.."

// table returns the declared rows of the module and the rows of 04 4.4.
func table(t *testing.T) ([]cli.Declared, []cli.SpecRow) {
	t.Helper()
	declared, err := cli.ReadTable(os.DirFS(root))
	require.NoError(t, err)
	src, err := os.ReadFile(root + "/docs/spec/04-cli.md")
	require.NoError(t, err)
	spec, err := cli.SpecRows(src)
	require.NoError(t, err)
	return declared, spec
}

// TestErrorIDsUnion is the union test of 04 4.1, Error ids: the rows the
// packages declare hold to 04 4.4.
func TestErrorIDsUnion(t *testing.T) {
	declared, spec := table(t)
	assert.Empty(t, cli.UnionProblems(declared, spec))
	assert.Contains(t, declared, cli.Declared{Row: cli.Usage, File: "internal/cli/errors.go"})
	// check-findings is the one id that maps to exit 6 (04 4.1, Exit
	// codes).
	for _, s := range spec {
		assert.Equal(t, s.Slug == "check-findings", s.Exit == 6, "%s %s exit %d", s.ID, s.Slug, s.Exit)
	}
}

// TestErrorIDsPending lists, with -v, the ids of 04 4.4 that no package
// declares yet; it fails on none until T073 (10 10.1).
func TestErrorIDsPending(t *testing.T) {
	declared, spec := table(t)
	for _, id := range cli.Pending(declared, spec) {
		t.Log("pending:", id)
	}
}

// TestUnionProblems holds each rule of the union test on a fixture.
func TestUnionProblems(t *testing.T) {
	spec := []cli.SpecRow{{ID: "RJ-301", Slug: "a", Exit: 3}, {ID: "RJ-309", Slug: "old", Exit: 2, Retired: true}}
	tests := []struct {
		name string
		rows []cli.Declared
		want string
	}{
		{"a duplicate id", []cli.Declared{{Row: cli.Row{ID: "RJ-301", Slug: "a", Exit: 3}, File: "x"}, {Row: cli.Row{ID: "RJ-301", Slug: "a", Exit: 3}, File: "y"}}, "y: RJ-301 is declared again, first in x"},
		{"an id not in 4.4", []cli.Declared{{Row: cli.Row{ID: "RJ-777", Slug: "b", Exit: 1}, File: "x"}}, "x: RJ-777 is not a row of 04 4.4"},
		{"a retired id", []cli.Declared{{Row: cli.Row{ID: "RJ-309", Slug: "old", Exit: 2}, File: "x"}}, "x: RJ-309 is retired in 04 4.4"},
		{"another slug", []cli.Declared{{Row: cli.Row{ID: "RJ-301", Slug: "c", Exit: 3}, File: "x"}}, "x: RJ-301 is c exit 3, but 04 4.4 says a exit 3"},
		{"another exit", []cli.Declared{{Row: cli.Row{ID: "RJ-301", Slug: "a", Exit: 1}, File: "x"}}, "x: RJ-301 is a exit 1, but 04 4.4 says a exit 3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, []string{tt.want}, cli.UnionProblems(tt.rows, spec))
		})
	}
	assert.Equal(t, []string{"04 4.4 lists RJ-301 twice"}, cli.UnionProblems(nil, append(spec, spec[0])))
	assert.Equal(t, []string{"RJ-301"}, cli.Pending(nil, spec), "a retired id is never pending")
}

// TestReadTable reads Row literals from errors.go files only, and refuses
// a field that is not a literal.
func TestReadTable(t *testing.T) {
	fsys := fstest.MapFS{
		"internal/a/errors.go": {Data: []byte("package a\nimport \"x/cli\"\nvar E = cli.Row{ID: \"RJ-301\", Slug: \"a\", Exit: 3, Fix: \"decide: x\", RecoveredIn: \"none: y\"}\n")},
		"internal/a/other.go":  {Data: []byte("package a\nimport \"x/cli\"\nvar F = cli.Row{ID: \"RJ-302\"}\n")},
	}
	got, err := cli.ReadTable(fsys)
	require.NoError(t, err)
	assert.Equal(t, []cli.Declared{{Row: cli.Row{ID: "RJ-301", Slug: "a", Exit: 3, Fix: "decide: x", RecoveredIn: "none: y"}, File: "internal/a/errors.go"}}, got)
	fsys["internal/b/errors.go"] = &fstest.MapFile{Data: []byte("package b\nimport \"x/cli\"\nconst id = \"RJ-303\"\nvar E = cli.Row{ID: id}\n")}
	_, err = cli.ReadTable(fsys)
	require.ErrorContains(t, err, "the field ID of a Row is a literal")
}

// fixedHint is the one fixed hint of 04 4.4, that of RJ-338
// (fingerprint-unstable), read from the page with its code-span
// backticks stripped.
func fixedHint(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile(root + "/docs/spec/04-cli.md")
	require.NoError(t, err)
	_, rest, ok := strings.Cut(string(src), "The hint is one fixed text: \"")
	require.True(t, ok)
	text, _, ok := strings.Cut(rest, "\"\n")
	require.True(t, ok)
	return strings.ReplaceAll(strings.Join(strings.Fields(text), " "), "`", "")
}

// programs are the programs a fix hint's command line may start with.
var programs = []string{"romeu", "julieta", "sbx", "git", "mise", "<binary>"}

// discards matches the clause on what a flag discards.
var discards = regexp.MustCompile(`\b(discards?|loses?|drops?|overwrites?)\b`)

// TestFixHints holds the hint rules of 04 4.1, Error ids: the one fixed
// hint of RJ-338 byte for byte, every other hint starting with a command
// line or "decide:", a hint naming a destructive flag saying what it
// discards, and a "recovered in" cell on each row.
func TestFixHints(t *testing.T) {
	declared, _ := table(t)
	want := fixedHint(t)
	assert.NotContains(t, want, "`")
	for _, d := range declared {
		// No package declares RJ-338 until its command lands; the check
		// runs from then.
		if d.ID == "RJ-338" {
			assert.Equal(t, want, d.Fix, "the hint of RJ-338 is the fixed text of 04 4.4")
			continue
		}
		first, _, _ := strings.Cut(d.Fix, " ")
		assert.True(t, strings.HasPrefix(d.Fix, "decide: ") || slices.Contains(programs, first), "%s: the hint %q starts with a command line or decide:", d.ID, d.Fix)
		for _, flagName := range []string{"--overwrite-drift", "--accept-loss", "--force"} {
			if strings.Contains(d.Fix, flagName) {
				assert.Regexp(t, discards, d.Fix, "%s: the hint names %s and says what it discards", d.ID, flagName)
			}
		}
		assert.Regexp(t, `^(docs/guide/[^#\s]+#\S+|none: \S.*)$`, d.RecoveredIn, "%s: the recovered-in cell", d.ID)
		// The reference page prints the hint in a code span and both cells
		// in a table row.
		assert.NotContains(t, d.Fix+d.RecoveredIn, "`", "%s: a hint and a recovered-in cell hold no backtick", d.ID)
		assert.NotContains(t, d.Fix+d.RecoveredIn, "\n", "%s: a hint and a recovered-in cell are one line", d.ID)
	}
}

// TestHintCommandsExist parses each romeu or julieta command line of a
// fix hint (but RJ-338's) and of a --help example into its command and
// flags, and fails on one the registered definitions do not have (04
// 4.1, Error ids, item 6).
func TestHintCommandsExist(t *testing.T) {
	declared, _ := table(t)
	var lines []string
	for _, d := range declared {
		if d.ID != "RJ-338" {
			lines = append(lines, d.Fix)
		}
	}
	for _, binary := range []string{"romeu", "julieta"} {
		for _, c := range cli.Commands(binary) {
			lines = append(lines, c.Examples...)
		}
	}
	for _, line := range lines {
		words := strings.Fields(line)
		if len(words) < 2 || (words[0] != "romeu" && words[0] != "julieta") || strings.HasPrefix(words[1], "-") {
			continue
		}
		c, ok := find(words[0], words[1])
		if !assert.True(t, ok, "%q names a command the definitions do not have", line) {
			continue
		}
		fs := flag.NewFlagSet(c.Name, flag.ContinueOnError)
		fs.Bool("help", false, "")
		if c.JSON {
			fs.Bool("json", false, "")
		}
		if c.Flags != nil {
			c.Flags(fs)
		}
		for _, w := range words[2:] {
			name, _, _ := strings.Cut(strings.TrimLeft(w, "-"), "=")
			if strings.HasPrefix(w, "-") && !strings.HasPrefix(name, "<") {
				assert.NotNil(t, fs.Lookup(name), "%q names the flag %s that %s %s does not have", line, w, c.Binary, c.Name)
			}
		}
	}
}

// find returns the registered definition of binary's command name.
func find(binary, name string) (cli.Command, bool) {
	for _, c := range cli.Commands(binary) {
		if c.Name == name {
			return c, true
		}
	}
	return cli.Command{}, false
}

// warnings returns the id-less rows of the list of 04 4.1, Warnings.
func warnings(t *testing.T) []string {
	t.Helper()
	src, err := os.ReadFile(root + "/docs/spec/04-cli.md")
	require.NoError(t, err)
	_, rest, ok := strings.Cut(string(src), "04 4.2, 04 4.3 and 08 8.2 cite it: ")
	require.True(t, ok)
	list, _, ok := strings.Cut(rest, " |")
	require.True(t, ok)
	var slugs []string
	for _, m := range regexp.MustCompile("`([a-z0-9-]+)`").FindAllStringSubmatch(list, -1) {
		slugs = append(slugs, m[1])
	}
	require.Contains(t, slugs, "sbx-untested")
	return slugs
}

// TestWarningSlugs reads every call of cli.Warn in the module's Go
// files and fails a slug that is not a literal of the list of 04 4.1,
// Warnings.
func TestWarningSlugs(t *testing.T) {
	known := warnings(t)
	fset := token.NewFileSet()
	err := fs.WalkDir(os.DirFS(root), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (strings.HasPrefix(d.Name(), ".") || d.Name() == "testdata") && p != "." {
			return fs.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		file, err := parser.ParseFile(fset, root+"/"+p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isWarnCall(call) || len(call.Args) < 2 {
				return true
			}
			lit, ok := call.Args[1].(*ast.BasicLit)
			if !assert.True(t, ok && lit.Kind == token.STRING, "%s: the slug of a warning is a literal", fset.Position(call.Pos())) {
				return true
			}
			slug, err := strconv.Unquote(lit.Value)
			require.NoError(t, err)
			assert.Contains(t, known, slug, "%s: the warning %s is no row of 04 4.1, Warnings", fset.Position(call.Pos()), slug)
			return true
		})
		return nil
	})
	require.NoError(t, err)
}

// isWarnCall reports whether call is cli.Warn(...).
func isWarnCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Warn" {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == "cli"
}
