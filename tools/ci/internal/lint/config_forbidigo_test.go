// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lint

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// boundaryCase is one forbidden call, written as the body of a function
// in a package that Allowed does not name and again in one it does.
type boundaryCase struct {
	name    string
	imports []string
	body    string
	// symbol is what the finding names, as "os.Stdout"; empty when the
	// call is no violation and no package may draw a finding for it.
	symbol string
}

// boundaryCases holds a violation for each pattern of forbidigo.yml, a
// renamed import, which only analyze-types reads, and two calls that
// look like a violation and are none.
var boundaryCases = []boundaryCase{
	{"fmtprintln", []string{`"fmt"`}, `fmt.Println("x")`, "fmt.Println"},
	{"fmtprintf", []string{`"fmt"`}, `fmt.Printf("x")`, "fmt.Printf"},
	{"osstdout", []string{`"os"`}, `_ = os.Stdout`, "os.Stdout"},
	{"osstderr", []string{`"os"`}, `_ = os.Stderr`, "os.Stderr"},
	{"logprintln", []string{`"log"`}, `log.Println("x")`, "log.Println"},
	{"texttemplate", []string{`"text/template"`}, `_ = template.New("x")`, "template.New"},
	{"renamedimport", []string{`o "os"`}, `_ = o.Stdout`, "o.Stdout"},
	// fmt.Fprintln to a writer the caller holds is the CLI writer's way.
	{"fprintln", []string{`"fmt"`, `"io"`}, `fmt.Fprintln(io.Discard, "x")`, ""},
	// A variable called log is no log package (pkg: ^log$).
	{"locallog", nil, "var log interface{ Println(...any) }\n\tif log != nil {\n\t\tlog.Println(\"x\")\n\t}", ""},
}

// source writes the case as a file of the package pkg; a test file
// holds it in a test function.
func (c boundaryCase) source(pkg string, test bool) string {
	imports, sig := c.imports, "func F_"+c.name+"() {"
	if test {
		imports, sig = append([]string{`"testing"`}, imports...), "func TestF_"+c.name+"(t *testing.T) {"
	}
	var b strings.Builder
	b.WriteString("package " + pkg + "\n\n")
	if len(imports) > 0 {
		b.WriteString("import (\n\t" + strings.Join(imports, "\n\t") + "\n)\n\n")
	}
	b.WriteString(sig + "\n\t" + c.body + "\n}\n")
	return b.String()
}

// TestLintBoundariesReportsOutsideAllowed runs the second lint run, as
// the lint step builds it (BoundaryArgs), over a module that holds each
// boundaryCase in a package outside Allowed and in each package inside
// it, and holds forbidigo to report the first and only the first. So the
// scope of the Boundaries patterns is the list of packages, and a
// pattern that stops reporting, or a package that stops being judged or
// stops being admitted, fails here.
func TestLintBoundariesReportsOutsideAllowed(t *testing.T) {
	files := map[string]string{}
	for _, c := range boundaryCases {
		files["internal/other/"+c.name+".go"] = c.source("other", false)
		// The test files of a package follow the package.
		files["internal/other/"+c.name+"_test.go"] = c.source("other", true)
		// A package below an allowed one is not an allowed one.
		files["internal/cli/sub/"+c.name+".go"] = c.source("sub", false)
		for _, dir := range Allowed {
			files[dir+"/"+c.name+".go"] = c.source(filepath.Base(dir), false)
			files[dir+"/"+c.name+"_test.go"] = c.source(filepath.Base(dir), true)
		}
	}
	// An Example function prints to stdout by design: forbidigo leaves
	// it out (exclude-godoc-examples, default true).
	files["internal/other/example_test.go"] = "package other\n\nimport \"fmt\"\n\nfunc ExampleF() {\n\tfmt.Println(\"x\")\n\t// Output: x\n}\n\nfunc F() {}\n"
	byFile := lintFixtureModule(t, files, func(dir string) []string {
		args, err := BoundaryArgs(dir)
		require.NoError(t, err)
		require.NotNil(t, args)
		at := slices.Index(args, BoundaryConfig)
		require.NotEqual(t, -1, at)
		args[at] = filepath.Join(citest.ModuleRoot(t), filepath.FromSlash(BoundaryConfig))
		return args
	})

	judged := map[string]bool{}
	for _, c := range boundaryCases {
		for _, file := range []string{"internal/other/" + c.name + ".go", "internal/other/" + c.name + "_test.go", "internal/cli/sub/" + c.name + ".go"} {
			judged[file] = true
			got := byFile[file]
			if c.symbol == "" {
				assert.Empty(t, got, "%s is no violation", file)
				continue
			}
			assert.True(t, slices.ContainsFunc(got, func(g string) bool {
				return strings.HasPrefix(g, "forbidigo: ") && strings.Contains(g, "use of `"+c.symbol+"` forbidden")
			}), "%s: want a forbidigo finding for %s, got %q", file, c.symbol, got)
		}
		for _, dir := range Allowed {
			for _, file := range []string{dir + "/" + c.name + ".go", dir + "/" + c.name + "_test.go"} {
				judged[file] = true
				assert.Empty(t, byFile[file], "%s is in an allowed package and is not judged", file)
			}
		}
	}
	judged["internal/other/example_test.go"] = true
	assert.Empty(t, byFile["internal/other/example_test.go"], "an Example function is left out")
	for file := range byFile {
		assert.True(t, judged[file], "a finding in %s, which is no fixture", file)
	}
}

// TestLintBoundaryConfig holds forbidigo.yml to the same limits as
// .golangci.yml (ADR 0007 rule 8): forbidigo is the one linter, nothing
// is excluded, generated files are read, and the patterns are the
// Boundaries ones, each tied to its standard package.
func TestLintBoundaryConfig(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), filepath.FromSlash(BoundaryConfig)))
	require.NoError(t, err)
	var cfg struct {
		Version string
		Linters struct {
			Default    string
			Enable     []string
			Exclusions map[string]any
			Settings   struct {
				Forbidigo struct {
					AnalyzeTypes bool `yaml:"analyze-types"`
					Forbid       []struct {
						Pattern, Pkg, Msg string
					}
				}
			}
		}
	}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	require.NoError(t, dec.Decode(&cfg), "no key beside these: no run, issues, output or formatters section, no other linter setting")
	assert.Equal(t, "2", cfg.Version)
	assert.Equal(t, "none", cfg.Linters.Default, "forbidigo alone")
	assert.Equal(t, []string{"forbidigo"}, cfg.Linters.Enable)
	assert.Equal(t, map[string]any{"generated": "disable"}, cfg.Linters.Exclusions, "the linters read generated files")
	fb := cfg.Linters.Settings.Forbidigo
	assert.True(t, fb.AnalyzeTypes, "a renamed import is read as what it is")
	var got [][2]string
	for _, f := range fb.Forbid {
		assert.NotEmpty(t, f.Msg, "the finding says why: %s", f.Pattern)
		got = append(got, [2]string{f.Pattern, f.Pkg})
	}
	assert.Equal(t, [][2]string{
		{`^fmt\.Print(f|ln)?$`, `^fmt$`},
		{`^os\.(Stdout|Stderr)$`, `^os$`},
		{`^log\..*$`, `^log$`},
		{`^template\..*$`, `^text/template$`},
	}, got, "the output patterns of item 16 of Decisions; exec.Command( is held by the call-site test of internal/procgroup, \"sh\", \"-c\" by TestNoShellCommands")
}

// TestBoundaryDirsOfTheModule holds the allow list to the module: the
// packages the second run judges are many, and none of them is in
// Allowed, so a list that grew to cover the whole module, or a walk that
// stopped finding packages, fails here and not as a lint step that
// judges nothing.
func TestBoundaryDirsOfTheModule(t *testing.T) {
	root := citest.ModuleRoot(t)
	dirs, err := BoundaryDirs(root)
	require.NoError(t, err)
	require.NotEmpty(t, dirs, "the second lint run has packages to judge")
	assert.Contains(t, dirs, "./tools/ci/internal/lint", "this package is judged")
	for _, allowed := range Allowed {
		assert.DirExists(t, filepath.Join(root, filepath.FromSlash(allowed)), "Allowed names a package that exists")
		assert.NotContains(t, dirs, "./"+allowed)
	}
}
