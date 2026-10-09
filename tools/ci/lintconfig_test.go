// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// lintFixture is one file of the fixture package: the source, and the
// finding the committed configuration must report for it. A file with
// no linter holds no violation and must stay clean.
type lintFixture struct {
	file    string
	source  string
	linter  string // the linter that reports it
	message string // a part of the message
}

// testifyViolation is one violation of one testifylint checker: the
// imports the body needs besides "testing", and the body of a test
// function. testifylint names the checker in the message of its
// finding, which is what the fixture test looks for.
type testifyViolation struct {
	checker string
	imports []string
	body    string
}

// testifyViolations holds one violation for each testifylint checker
// that applies outside suites, the list of the README of testifylint
// v1.6.4, which the pinned golangci-lint bundles. The suite-* checkers
// have no fixture here: the repository's tests use no suite (ADR
// 0007), and TestLintConfigLowersNothing checks "enable-all: true",
// which turns them on.
var testifyViolations = []testifyViolation{
	{"blank-import", []string{`_ "github.com/stretchr/testify/assert"`}, ``},
	{"bool-compare", []string{assertImport}, `assert.Equal(t, true, len(os.Args) > 0)`},
	{"compares", []string{assertImport}, `assert.True(t, len(os.Args) == 1)`},
	{"contains", []string{assertImport, `"strings"`}, `assert.True(t, strings.Contains("abc", "b"))`},
	{"empty", []string{assertImport}, `assert.Len(t, os.Args, 0)`},
	{"encoded-compare", []string{assertImport}, "var expectedJSON, actualJSON string\n\tassert.Equal(t, expectedJSON, actualJSON)"},
	{"equal-values", []string{assertImport}, "a, b := 1, 2\n\tassert.EqualValues(t, a, b)"},
	{"error-is-as", []string{assertImport, `"errors"`, `"io"`}, `assert.True(t, errors.Is(io.ErrUnexpectedEOF, io.EOF))`},
	{"error-nil", []string{assertImport, `"io"`}, `assert.Nil(t, io.EOF)`},
	{"expected-actual", []string{assertImport}, `assert.Equal(t, len(os.Args), 1)`},
	{"float-compare", []string{assertImport}, `assert.Equal(t, 1.5, float64(len(os.Args)))`},
	{"formatter", []string{assertImport, `"fmt"`}, "a, b := 1, 2\n\tassert.Equal(t, a, b, fmt.Sprintf(\"args %d\", a))"},
	{"go-require", []string{requireImport}, `go func() { require.NotNil(t, os.Args) }()`},
	{"len", []string{assertImport}, `assert.Equal(t, 1, len(os.Args))`},
	{"negative-positive", []string{assertImport}, "a := 1\n\tassert.Less(t, a, 0)"},
	{"nil-compare", []string{assertImport}, `assert.Equal(t, nil, os.Args)`},
	{"regexp", []string{assertImport, `"regexp"`}, `assert.Regexp(t, regexp.MustCompile("a"), "a")`},
	{"require-error", []string{assertImport, `"io"`}, "assert.Error(t, io.EOF)\n\tassert.Equal(t, \"EOF\", io.EOF.Error())"},
	{"useless-assert", []string{assertImport}, `assert.Equal(t, os.Args, os.Args)`},
}

const (
	assertImport  = `"github.com/stretchr/testify/assert"`
	requireImport = `"github.com/stretchr/testify/require"`
)

// source writes the violation as the file of a fixture package. Every
// body may use "os", so the import is added when a body names it.
func (v testifyViolation) source() string {
	imports := append([]string{`"testing"`}, v.imports...)
	if strings.Contains(v.body, "os.") {
		imports = append(imports, `"os"`)
	}
	return "package fixture\n\nimport (\n\t" + strings.Join(imports, "\n\t") +
		"\n)\n\nfunc Test_" + strings.ReplaceAll(v.checker, "-", "_") + "(t *testing.T) {\n\t" + v.body + "\n}\n"
}

// lintFixtures holds one violation of each checker of ADR 0001, rule
// 14, one of each testifylint checker of testifyViolations, one per
// file, and a clean file as the control.
func lintFixtures() []lintFixture {
	fixtures := []lintFixture{
		{
			file:    "doccomment.go",
			source:  "package fixture\n\nfunc Undocumented() {}\n",
			linter:  "revive",
			message: "exported function Undocumented should have comment",
		},
		{
			// Several words that end with a full stop: revive's
			// error-strings and staticcheck's ST1005 both report it.
			// revive alone skips a one-word capitalized string.
			file: "errorstring.go",
			source: `package fixture

import "errors"

func errorString() error {
	return errors.New("Capitalized and ends with a full stop.")
}

var _ = errorString
`,
			linter:  "revive",
			message: "error strings should not be capitalized",
		},
		{
			file: "commentedcode.go",
			source: `package fixture

func commentedCode(n int) int {
	// total := n * 2
	// if total > 10 { return total }
	return n
}

var _ = commentedCode
`,
			linter:  "gocritic",
			message: "may want to remove commented-out code",
		},
	}
	for _, v := range testifyViolations {
		fixtures = append(fixtures, lintFixture{
			file:    strings.ReplaceAll(v.checker, "-", "") + "_test.go",
			source:  v.source(),
			linter:  "testifylint",
			message: v.checker + ": ",
		})
	}
	return append(fixtures, lintFixture{
		file: "clean.go",
		source: `package fixture

// Documented has a doc comment, an error string in Go style and no
// commented-out code.
func Documented() error {
	return nil
}
`,
	})
}

// goFileInError matches a file name with a line number in a compiler
// error, as "errorstring.go:5:2".
var goFileInError = regexp.MustCompile(`[\w.-]+\.go:\d+`)

// moduleRoot returns the root of this repository.
func moduleRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	return root
}

// TestLintConfigReportsEachChecker runs the pinned golangci-lint with
// the committed .golangci.yml over a fixture package that holds one
// violation of each checker of ADR 0001, rule 14 (doc comments, error
// strings, commented-out code) and of each testifylint checker of
// testifyViolations, so a configuration that stops reporting one of
// them fails the build (10 10.1). The fixtures prove those checkers;
// the suite-* checkers of testifylint are covered by
// TestLintConfigLowersNothing.
func TestLintConfigReportsEachChecker(t *testing.T) {
	root := moduleRoot(t)
	config := filepath.Join(root, ".golangci.yml")
	require.FileExists(t, config)
	fixtures := lintFixtures()

	// The fixture is a module of its own, with the dependencies of
	// this one, so it needs the module cache this module needs and no
	// network. It runs in the environment of the lint step (lintEnv),
	// for the first of lintTargets: the fixture files build for each.
	dir := t.TempDir()
	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o644))
	}
	for _, f := range fixtures {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f.file), []byte(f.source), 0o644))
	}

	// The tools are the ones the lint step runs (resolveLintTools), and
	// the run happens in the fixture's directory, where no mise.toml is.
	ctx, cancel := context.WithTimeout(t.Context(), stepTimeout)
	defer cancel()
	tools := pinnedLintTools(t)

	report := filepath.Join(t.TempDir(), "report.json")
	cmd := exec.CommandContext(ctx, tools.linter, "run",
		"--config", config, "--output.json.path", report, "--output.text.path", os.DevNull)
	cmd.Dir = dir
	cmd.Env = lintEnv(lintTargets[0], tools.goDir)
	out, err := cmd.CombinedOutput()
	// Exit status 1 means the run found issues, which is the point.
	var exit *exec.ExitError
	if err != nil {
		require.ErrorAs(t, err, &exit, "golangci-lint run\n%s", out)
		require.Equal(t, 1, exit.ExitCode(), "exit status of golangci-lint run\n%s", out)
	}

	data, err := os.ReadFile(report)
	require.NoError(t, err, "golangci-lint wrote no report\n%s", out)
	var parsed struct {
		Issues []struct {
			FromLinter string
			Text       string
			Pos        struct{ Filename string }
		}
	}
	require.NoError(t, json.Unmarshal(data, &parsed))

	byFile := map[string][]string{} // file -> "linter: text"
	for _, is := range parsed.Issues {
		// A fixture that does not compile is no fixture: the linters
		// report nothing for it.
		// The position of a typecheck finding is the first file of the
		// package; the file that is broken is the one the text names.
		require.NotEqual(t, "typecheck", is.FromLinter, "a fixture does not compile: %s\n%s",
			strings.Join(goFileInError.FindAllString(is.Text, -1), " "), is.Text)
		file := filepath.Base(is.Pos.Filename)
		byFile[file] = append(byFile[file], is.FromLinter+": "+is.Text)
	}
	judged := map[string]bool{}
	for _, f := range fixtures {
		judged[f.file] = true
		got := byFile[f.file]
		if f.linter == "" {
			assert.Empty(t, got, "%s is the control and must stay clean", f.file)
			continue
		}
		// A violation file may draw findings from other linters too
		// (staticcheck also reports the error string); one finding of
		// the right linter with the right text is what must exist.
		want := f.linter + ": "
		found := slices.ContainsFunc(got, func(g string) bool {
			return strings.HasPrefix(g, want) && strings.Contains(g, f.message)
		})
		assert.True(t, found, "%s: want a finding %q with %q, got %q", f.file, want, f.message, got)
	}
	// A finding in a file that is none of the fixtures fails by design,
	// and so does one in the control: a default linter that starts to
	// report on a fixture file is a change to review.
	for file := range byFile {
		assert.True(t, judged[file], "a finding in %s, which is no fixture", file)
	}
}

// TestLintConfigLowersNothing reads .golangci.yml and holds it to ADR
// 0007 (rules 7 and 8): testifylint with every checker, which covers
// the suite-* ones; and nothing that hides a finding. Every key of the
// file is on a short list, so a new one (an exclusion, a disabled
// linter, a plugin, a limit on the issues, a path or the tests left
// out of the run) is a decision the reader of this test sees.
func TestLintConfigLowersNothing(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(moduleRoot(t), ".golangci.yml"))
	require.NoError(t, err)
	var cfg map[string]any
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	keys := func(v any) []string {
		m, ok := v.(map[string]any)
		require.True(t, ok, "a mapping, got %T", v)
		var ks []string
		for k := range m {
			ks = append(ks, k)
		}
		return ks
	}
	assert.ElementsMatch(t, []string{"version", "linters"}, keys(cfg), "top-level keys: no issues, run, output or formatters section")

	linters, ok := cfg["linters"].(map[string]any)
	require.True(t, ok, "linters")
	assert.ElementsMatch(t, []string{"enable", "settings"}, keys(linters),
		"linters keys: no default, disable, exclusions")
	settings, ok := linters["settings"].(map[string]any)
	require.True(t, ok, "linters.settings")
	assert.ElementsMatch(t, []string{"revive", "gocritic", "testifylint"}, keys(settings),
		"linters.settings keys: no custom linter")

	assert.Equal(t, map[string]any{"enable-all": true}, settings["testifylint"], "testifylint runs every checker")
	revive, ok := settings["revive"].(map[string]any)
	require.True(t, ok, "linters.settings.revive")
	assert.ElementsMatch(t, []string{"enable-all-rules", "rules"}, keys(revive))
	// enable-all-rules would turn on every rule of revive, each one a
	// finding of its own that ADR 0001 never asked for.
	all, ok := revive["enable-all-rules"].(bool)
	require.True(t, ok, "revive enable-all-rules is a boolean")
	assert.False(t, all, "revive enable-all-rules")
	rules, ok := revive["rules"].([]any)
	require.True(t, ok, "revive rules")
	var names []string
	for _, r := range rules {
		assert.Equal(t, []string{"name"}, keys(r), "a revive rule is its name alone: no severity, no disabled, no arguments")
		name, _ := r.(map[string]any)["name"].(string)
		names = append(names, name)
	}
	assert.Equal(t, []string{"exported", "error-strings"}, names, "the revive rules of ADR 0001, rule 14")
}

// allowedDirectives are the directives a comment of any Go file of this
// repository may hold, as "tool:name". go:build is the build
// constraint (add_test.go). Every other directive, of any tool, is a
// decision to review here first, because the readers of directives
// include the linters, and theirs turn findings off.
var allowedDirectives = []string{"go:build"}

// toolDirective matches the start of a directive of the "tool:name"
// form after directiveText has normalised the line: the shape go/ast
// reads (isDirective in go/ast/ast.go, Go 1.27: "[a-z0-9]+:[a-z0-9]"),
// which revive's "revive:disable" and staticcheck's "lint:ignore" have.
// The match is the "tool:name" word.
var toolDirective = regexp.MustCompile(`^[a-z0-9]+:[a-z0-9]\S*`)

// nolintDirective matches golangci-lint's own directive after
// directiveText. golangci-lint matches "^nolint( |:|$)"
// (NewNolintFilter, pkg/result/processors/nolint_filter.go, v2.14.0);
// this one also takes the word followed by any other character that
// ends it, which is more than the linter reads. It is built in two
// parts so that this file does not hold what it looks for.
var nolintDirective = regexp.MustCompile(`^no` + `lint\b`)

// goLineDirectives are the directives without a colon that the Go
// toolchain reads when they follow "//" or "/*" with no space between
// (isDirective in go/ast/ast.go): a line directive, which moves the
// position of what follows (and golangci-lint drops a finding moved to
// a file that is not Go), gccgo's extern and cgo's export.
var goLineDirectives = []string{"line", "extern", "export"}

// directiveText normalises one line of a comment as the most lenient
// reader of directives does. golangci-lint applies
// strings.TrimLeft(text, "/ ") to the text of the comment with its "//"
// or "/*" (extractInlineRangeFromComment of nolint_filter.go), and
// revive accepts "\s", any of tab, line feed, form feed, carriage return
// and space, after "//" (directiveRegexp in lint/file.go, v1.17.0). This
// one trims "/", "*" and every rune for which unicode.IsSpace holds, a
// superset of both sets, so it finds at least what each finds.
func directiveText(line string) string {
	return strings.TrimLeftFunc(line, func(r rune) bool {
		return r == '/' || r == '*' || unicode.IsSpace(r)
	})
}

// directives returns the directives that the comments of the Go source
// hold, as "tool:name" or the bare name, in any position: every line of
// every comment read leniently (directiveText) for the "tool:name" form
// and golangci-lint's, and the start of each comment read as the Go
// toolchain reads it for the forms of goLineDirectives.
func directives(src []byte) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var found []string
	for _, group := range f.Comments {
		for _, c := range group.List {
			// c.Text starts with "//" or "/*", and the toolchain reads
			// the word right after it.
			for _, name := range goLineDirectives {
				if strings.HasPrefix(c.Text[2:], name+" ") {
					found = append(found, name)
				}
			}
			for line := range strings.SplitSeq(c.Text, "\n") {
				text := directiveText(line)
				if m := toolDirective.FindString(text); m != "" {
					found = append(found, m)
				} else if m := nolintDirective.FindString(text); m != "" {
					found = append(found, m)
				}
			}
		}
	}
	return found, nil
}

// generatedMarkers are the markers that golangci-lint v2.14.0 treats as
// the sign of a generated file in its default "lax" mode, in lower case
// (isGeneratedFileLax in pkg/result/processors/
// exclusion_generated_file_matcher.go). They are built in parts so that
// this file does not hold what it looks for.
var generatedMarkers = []string{
	"code gener" + "ated",
	"do not ed" + "it",
	"autogener" + "ated file",
	"* gener" + "ated by: swagger codegen ",
}

// isGenerated reports whether the Go source holds a comment that
// golangci-lint would take for a generated-code marker, which makes it
// skip the file. The linter looks at the comments of the file in any
// position, in line and block comments, so this does too, and it reads
// the raw comment text, which holds at least what the linter reads.
func isGenerated(src []byte) (bool, error) {
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return false, err
	}
	for _, group := range f.Comments {
		for _, c := range group.List {
			text := strings.ToLower(c.Text)
			for _, marker := range generatedMarkers {
				if strings.Contains(text, marker) {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// TestIsGenerated pins isGenerated on the shapes of the header, in any
// position of the file, as the linter reads them. The
// header is built in parts so that this file is not one.
func TestIsGenerated(t *testing.T) {
	header := "// Code gener" + "ated by x. DO NOT ED" + "IT."
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{"header on line 1", header + "\npackage p\n", true},
		{"after a build constraint", "//go:build linux\n\n" + header + "\n\npackage p\n", true},
		{"CRLF line ends", header + "\r\npackage p\r\n", true},
		{"byte order mark", "\ufeff" + header + "\npackage p\n", true},
		{"after the package clause", "package p\n\n" + header + "\n", true},
		{"at the end of the file", "package p\n\nfunc F() {}\n\n" + header + "\n", true},
		{"block comment", "package p\n\n/* " + header + " */\n", true},
		{"inside a doc comment", "package p\n\n// F is the one. Do not ed" + "it by hand.\nfunc F() {}\n", true},
		{"no header", "package p\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isGenerated([]byte(tt.src))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// repoFiles lists the files of the repository that match the pathspecs,
// the tracked ones and the ones not yet added, as paths below root. The
// ones that a .gitignore names are listed too: the linter reads them
// all the same.
func repoFiles(t *testing.T, root string, pathspecs ...string) []string {
	t.Helper()
	args := append([]string{"-C", root, "ls-files", "-z", "--cached", "--others", "--"}, pathspecs...)
	out, err := exec.CommandContext(t.Context(), "git", args...).Output()
	require.NoError(t, err, "git ls-files")
	var files []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, f)); errors.Is(err, os.ErrNotExist) {
			continue // listed and deleted in the working tree
		}
		files = append(files, f)
	}
	return files
}

// disallowed returns what the Go file at path holds that would make the
// linters skip a finding: each directive that is not one of allowed,
// and a generated-code header.
func disallowed(path string, allowed []string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	found, err := directives(data)
	if err != nil {
		return nil, err
	}
	var bad []string
	for _, name := range found {
		if !slices.Contains(allowed, name) {
			bad = append(bad, "directive "+name)
		}
	}
	generated, err := isGenerated(data)
	if err != nil {
		return nil, err
	}
	if generated {
		bad = append(bad, "a generated-code header")
	}
	return bad, nil
}

// TestDisallowed pins disallowed on files written to disk, the way
// TestOnlyAllowedDirectives reads the files of the repository. The
// directives are built in parts so that this file holds none.
func TestDisallowed(t *testing.T) {
	nl, rv := "no"+"lint", "revive:dis"+"able"
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"clean", "//go:build linux\n\n// Package p is it.\npackage p\n", nil},
		{"a nolint", "package p\n\nvar _ = 1 //" + nl + "\n", []string{"directive " + nl}},
		{"revive after a form feed", "package p\n\n// Doc.\n// \f" + rv + "\nfunc F() {}\n", []string{"directive " + rv}},
		{"a stray go:generate", "package p\n\n//go:generate echo hi\nvar _ = 1\n", []string{"directive go:generate"}},
		{"go:embed", "package p\n\n//go:embed data.txt\nvar data string\n", []string{"directive go:embed"}},
		{"go:linkname", "package p\n\n//go:linkname f runtime.f\nfunc f()\n", []string{"directive go:linkname"}},
		{"generated", "// Code gener" + "ated by x. DO NOT ED" + "IT.\n\npackage p\n", []string{"a generated-code header"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "a.go")
			require.NoError(t, os.WriteFile(path, []byte(tt.src), 0o644))
			got, err := disallowed(path, allowedDirectives)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestOnlyAllowedDirectives checks that each directive in the Go files
// of the repository, the tracked ones and the ones not yet added, is one
// of allowedDirectives, and that no file carries a generated-code
// header (ADR 0007, rule 8): a finding is fixed in the code, and a
// directive or a header that makes the linters skip it hides findings.
func TestOnlyAllowedDirectives(t *testing.T) {
	root := moduleRoot(t)
	files := repoFiles(t, root, "*.go")
	require.NotEmpty(t, files, "the Go files of the repository")
	for _, f := range files {
		allowed := allowedDirectives
		if f == generateSite {
			allowed = append(slices.Clone(allowed), "go:generate")
		}
		bad, err := disallowed(filepath.Join(root, f), allowed)
		require.NoError(t, err, "read %s", f)
		assert.Empty(t, bad, "%s holds what makes the linters skip a finding", f)
	}
	// The raw lines that go generate runs: exactly one in the repository,
	// at its site (directives.go). The comments are read above for the
	// other directives and here for the lenient forms of this one.
	problems, err := checkGenerateDirectives(root)
	require.NoError(t, err)
	assert.Empty(t, problems, "the go:generate lines of the repository")
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(generateSite)))
	require.NoError(t, err)
	found, err := directives(src)
	require.NoError(t, err)
	assert.Equal(t, 1, countEntries(found, "go:generate"), "%s", generateSite)
}

// countEntries returns how many entries of found are exactly name.
func countEntries(found []string, name string) int {
	n := 0
	for _, f := range found {
		if f == name {
			n++
		}
	}
	return n
}

// TestDirectiveCountIsPerEntry checks that the count of the go:generate
// directives of a file is a count of entries, so a longer directive
// such as go:generated does not pass for one.
func TestDirectiveCountIsPerEntry(t *testing.T) {
	found, err := directives([]byte("package p\n\n//go:generated x\n//go:generate y\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{"go:generated", "go:generate"}, found)
	assert.Equal(t, 1, countEntries(found, "go:generate"))
	found, err = directives([]byte("package p\n\n//go:generated x\n"))
	require.NoError(t, err)
	assert.Zero(t, countEntries(found, "go:generate"), "go:generated is another directive")
}

// TestGenerateProblems pins the rule of the one go:generate line on the
// forms that break it, among them the ones a reader of comments does
// not see and go generate runs: a line in a raw string or in a block
// comment.
func TestGenerateProblems(t *testing.T) {
	line := "//go:generate go run . generate"
	site := "package main\n\n" + line + "\n"
	tests := []struct {
		name  string
		files map[string]string
		ok    bool
	}{
		{"the directive", map[string]string{generateSite: site}, true},
		{"with trailing space", map[string]string{generateSite: site[:len(site)-1] + " \n"}, true},
		{"trailing spaces and tabs before a CRLF", map[string]string{generateSite: "package main\r\n\r\n" + line + " \t\r\n"}, true},
		{"a form feed at the end", map[string]string{generateSite: site[:len(site)-1] + "\f\n"}, false},
		{"a vertical tab at the end", map[string]string{generateSite: site[:len(site)-1] + "\v\n"}, false},
		{"a carriage return before a space is part of the word", map[string]string{generateSite: site[:len(site)-1] + "\r \n"}, false},
		{"a CRLF line", map[string]string{generateSite: "package main\r\n\r\n" + line + "\r\n"}, true},
		{"none", map[string]string{generateSite: "package main\n"}, false},
		{"no site file", map[string]string{"a.go": "package a\n"}, false},
		{"another command", map[string]string{generateSite: "package main\n\n//go:generate go run . generated\n"}, false},
		{"an added argument", map[string]string{generateSite: "package main\n\n" + line + " x\n"}, false},
		{"a tab after the word", map[string]string{generateSite: "package main\n\n//go:generate\tgo run . generate\n"}, false},
		{"twice", map[string]string{generateSite: site + line + "\n"}, false},
		{"a second command", map[string]string{generateSite: site + "\n// A.\n//go:generate echo hi\n"}, false},
		{"a second directive in a raw string of the site", map[string]string{generateSite: site + "\nvar s = `\n//go:generate echo hi\n`\n"}, false},
		{"a stray comment", map[string]string{generateSite: site, "a/a.go": "package a\n\n//go:generate echo hi\n"}, false},
		{"a stray in a raw string", map[string]string{generateSite: site, "zz/z.go": "package z\n\nvar s = `\n//go:generate echo hi\n`\n"}, false},
		{"a stray in a block comment", map[string]string{generateSite: site, "zz/z.go": "package z\n\n/*\n//go:generate echo hi\n*/\n"}, false},
		{"a stray with CRLF", map[string]string{generateSite: site, "zz/z.go": "package z\r\n\r\n//go:generate echo hi\r\n"}, false},
		{"the text is not a line start", map[string]string{generateSite: site, "zz/z.go": "package z\n\nvar s = \"//go:generate echo hi\"\n"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string][]byte{}
			for name, src := range tt.files {
				files[name] = []byte(src)
			}
			problems := generateProblems(files)
			assert.Equal(t, tt.ok, len(problems) == 0, "%v", problems)
		})
	}
}

// TestCheckGenerateDirectivesReadsEveryGoFile pins that the scan on disk
// reads what go generate would skip or might reach: testdata, vendor, a
// "_" directory, a file with a build constraint that excludes it, a
// test file and a link to a file; and leaves out the .git directories,
// a link to a directory and a dangling link.
func TestCheckGenerateDirectivesReadsEveryGoFile(t *testing.T) {
	stray := "package z\n\nvar s = `\n//go:generate echo hi\n`\n"
	site := "package main\n\n//go:generate go run . generate\n"
	for _, rel := range []string{
		"testdata/z.go", "vendor/z.go", "_x/z.go", ".hidden/z.go", "z_test.go",
		"z_windows.go", "sub/mod/z.go", "link.go",
	} {
		t.Run(rel, func(t *testing.T) {
			dir := t.TempDir()
			write := func(rel, src string) {
				require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, rel), []byte(src), 0o644))
			}
			write(generateSite, site)
			switch rel {
			case "link.go":
				write("real.txt", stray)
				require.NoError(t, os.Symlink("real.txt", filepath.Join(dir, rel)))
			case "sub/mod/z.go":
				write("sub/mod/go.mod", "module m\n")
				write(rel, stray)
			default:
				write(rel, stray)
			}
			problems, err := checkGenerateDirectives(dir)
			require.NoError(t, err)
			require.Len(t, problems, 1)
			assert.Equal(t, rel, problems[0].where)
		})
	}
	// What the scan leaves out: a .git directory at any depth (a nested
	// one, of a submodule or a vendored checkout, included), a dangling
	// link, and a link to a directory, which it does not enter: the
	// files of the target are read once, through the real path.
	t.Run("what the scan leaves out", func(t *testing.T) {
		dir := t.TempDir()
		write := func(rel, src string) {
			require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(dir, rel), []byte(src), 0o644))
		}
		write(generateSite, site)
		write(".git/z.go", stray)
		write("sub/.git/z.go", stray)
		require.NoError(t, os.Symlink("missing", filepath.Join(dir, "dangling.go")))
		write("real/ok.go", "package ok\n")
		require.NoError(t, os.Symlink("real", filepath.Join(dir, "dirlink")))
		require.NoError(t, os.Symlink("dirlink", filepath.Join(dir, "chain")))
		require.NoError(t, os.Symlink(".git", filepath.Join(dir, "gitlink")))
		problems, err := checkGenerateDirectives(dir)
		require.NoError(t, err)
		assert.Empty(t, problems)
	})
	t.Run("a link to a directory is not entered", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "real"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "real", "z.go"), []byte(stray), 0o644))
		require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(generateSite)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, generateSite), []byte(site), 0o644))
		require.NoError(t, os.Symlink("real", filepath.Join(dir, "dirlink")))
		problems, err := checkGenerateDirectives(dir)
		require.NoError(t, err)
		require.Len(t, problems, 1, "the target is read once, by its real path")
		assert.Equal(t, "real/z.go", problems[0].where)
	})
	t.Run("a link to a directory with a .go name is an error, not a pass", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "real"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(generateSite)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, generateSite), []byte(site), 0o644))
		require.NoError(t, os.Symlink("real", filepath.Join(dir, "dir.go")))
		_, err := checkGenerateDirectives(dir)
		require.Error(t, err)
	})
}

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
	root := moduleRoot(t)
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

// TestOneLintConfig checks that .golangci.yml is the only golangci-lint
// configuration file in the repository, the tracked ones and the ones
// not yet added: golangci-lint prefers a .golangci.yaml, .toml or .json
// beside it, and the lint step and the fixture test both name
// .golangci.yml, so a second file would be one that nothing reads.
func TestOneLintConfig(t *testing.T) {
	root := moduleRoot(t)
	var found []string
	for _, f := range repoFiles(t, root, "*.golangci.*") {
		if strings.HasPrefix(filepath.Base(f), ".golangci.") {
			found = append(found, f)
		}
	}
	assert.Equal(t, []string{".golangci.yml"}, found, "the golangci-lint configuration files of the repository")
}

// TestDirectives pins directives on the spellings that a reader of
// directives accepts, and on the comments that are none. The
// directives are built in parts so that this file holds none.
func TestDirectives(t *testing.T) {
	nl, ig := "no"+"lint", "lint:ig"+"nore"
	rv, ln := "revive:dis"+"able", "li"+"ne"
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"plain", "package p\n\nvar _ = 1 //" + nl + "\n", []string{nl}},
		{"space", "package p\n\nvar _ = 1 // " + nl + ":errcheck\n", []string{nl + ":errcheck"}},
		{"slash then name", "package p\n\nvar _ = 1 // /" + nl + "\n", []string{nl}},
		{"slash space name with linter", "package p\n\nvar _ = 1 // / " + nl + ":errcheck\n", []string{nl + ":errcheck"}},
		{"many slashes", "package p\n\nvar _ = 1 /////" + nl + "\n", []string{nl}},
		{"tab", "package p\n\nvar _ = 1 //\t" + nl + "\n", []string{nl}},
		{"revive after a tab", "package p\n\n//\t" + rv + "\nvar _ = 1\n", []string{rv}},
		{"revive after a form feed", "package p\n\n// \f" + rv + "\nvar _ = 1\n", []string{rv}},
		{"revive after a vertical tab", "package p\n\n//\v" + rv + "\nvar _ = 1\n", []string{rv}},
		{"revive after a carriage return", "package p\n\n/*\r" + rv + " */\nvar _ = 1\n", []string{rv}},
		{"revive after a no-break space", "package p\n\n//\u00a0" + rv + "\nvar _ = 1\n", []string{rv}},
		{"block comment", "package p\n\nvar _ = 1 /* " + nl + " */\n", []string{nl}},
		{"second line of a block comment", "package p\n\n/*\nabout\n" + nl + ":all\n*/\nvar _ = 1\n", []string{nl + ":all"}},
		{"staticcheck", "package p\n\n//" + ig + " SA1000 reason\nvar _ = 1\n", []string{ig}},
		{"staticcheck file", "package p\n\n// lint:file-ig" + "nore SA1000 reason\n", []string{"lint:file-ig" + "nore"}},
		{"in a doc comment line", "package p\n\n// F is it.\n//" + nl + "\nfunc F() {}\n", []string{nl}},
		{"revive", "package p\n\n//" + rv + "\nvar _ = 1\n", []string{rv}},
		{"revive with rule and space", "package p\n\n// " + rv + "-line:exported reason\nvar _ = 1\n", []string{rv + "-line:exported"}},
		{"line directive", "package p\n\n//" + ln + " x.txt:1\nvar _ = 1\n", []string{ln}},
		{"block line directive", "package p\n\nvar _ = /*" + ln + " x.txt:1:1*/ 1\n", []string{ln}},
		{"cgo export", "package p\n\n//exp" + "ort F\nfunc F() {}\n", []string{"exp" + "ort"}},
		{"gccgo extern", "package p\n\n//ext" + "ern c_f\nfunc F()\n", []string{"ext" + "ern"}},
		{"build constraint", "//go:build linux\n\npackage p\n", []string{"go:build"}},
		{"generate", "package p\n\n//go:gen" + "erate stringer\n", []string{"go:gen" + "erate"}},
		{"prose about it", "package p\n\n// F never uses a " + nl + " directive.\nfunc F() {}\n", nil},
		{"prose line that starts with the word line", "package p\n\n// the first\n// " + ln + " of it.\nvar _ = 1\n", nil},
		{"a URL", "package p\n\n// https://go.dev/doc/comment\nvar _ = 1\n", nil},
		{"a string", "package p\n\nvar _ = \"//" + nl + "\"\n", nil},
		{"a longer word", "package p\n\n// " + nl + "x is a word.\nvar _ = 1\n", nil},
		{"no comment", "package p\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := directives([]byte(tt.src))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// lintTargetContexts returns the build context of each of lintTargets,
// the list the lint step runs for, as the pinned go command builds it
// in the environment of the lint step (lintEnv). It asks "go list" for
// the context (the template function "context" of "go help list"), so
// the tool tags are the toolchain's own: the instruction set level
// (amd64.v1, arm64.v8.0) and the goexperiment tags, which differ from
// one target to another (cfg.go and internal/buildcfg in
// cmd/go, Go 1.27). Cgo is off, and there are no tags of our own.
func lintTargetContexts(t *testing.T) []build.Context {
	t.Helper()
	tools := pinnedLintTools(t)
	format := `{{context.CgoEnabled}} {{context.Compiler}} {{join context.ToolTags ","}} {{join context.ReleaseTags ","}}`
	var contexts []build.Context
	for _, target := range lintTargets {
		cmd := exec.CommandContext(t.Context(), filepath.Join(tools.goDir, "go"), "list", "-f", format, "runtime")
		cmd.Dir = t.TempDir()
		cmd.Env = lintEnv(target, tools.goDir)
		out, err := cmd.Output()
		require.NoError(t, err, "go list for %s", target)
		fields := strings.Fields(string(out))
		require.Len(t, fields, 4, "go list for %s: %q", target, out)
		contexts = append(contexts, build.Context{
			GOOS:        target.goos,
			GOARCH:      target.goarch,
			CgoEnabled:  fields[0] == "true",
			Compiler:    fields[1],
			ToolTags:    strings.Split(fields[2], ","),
			ReleaseTags: strings.Split(fields[3], ","),
		})
	}
	return contexts
}

// buildsForLintTarget reports whether the Go file builds for at least
// one of the contexts of lintTargetContexts. go/build applies the file-name suffixes (_darwin,
// _arm64, _test) and the "//go:build" lines through go/build/constraint,
// and ignores files that begin with "_" or ".". MatchFile leaves the
// import of "C" to Import, so this reads the imports too: a file that
// imports "C" builds only with cgo, which every target has off.
func buildsForLintTarget(contexts []build.Context, dir, name string) (bool, error) {
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.ImportsOnly)
	if err != nil {
		return false, err
	}
	for _, imp := range f.Imports {
		if imp.Path.Value == `"C"` {
			return false, nil
		}
	}
	for _, ctx := range contexts {
		ok, err := ctx.MatchFile(dir, name)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// TestBuildsForLintTarget pins buildsForLintTarget on constraints that
// no lint target satisfies and on those that one does.
func TestBuildsForLintTarget(t *testing.T) {
	tests := []struct {
		name string
		file string
		src  string
		want bool
	}{
		{"no constraint", "a.go", "package p\n", true},
		{"darwin only by name", "a_darwin.go", "package p\n", true},
		{"darwin arm64 test by name", "a_darwin_arm64_test.go", "package p\n", true},
		{"amd64 by name", "a_amd64.go", "package p\n", true},
		{"amd64 by tag", "a.go", "//go:build amd64\n\npackage p\n", true},
		{"cgo only", "a.go", "//go:build cgo\n\npackage p\n", false},
		{"not cgo", "a.go", "//go:build !cgo\n\npackage p\n", true},
		{"imports C", "a.go", "package p\n\nimport \"C\"\n", false},
		{"riscv64 by name", "a_riscv64.go", "package p\n", false},
		{"windows by name", "a_windows.go", "package p\n", false},
		{"linux 386 by name", "a_linux_386.go", "package p\n", false},
		{"ignore tag", "a.go", "//go:build ignore\n\npackage p\n", false},
		{"custom tag", "a.go", "//go:build never\n\npackage p\n", false},
		{"unix", "a.go", "//go:build unix\n\npackage p\n", true},
		{"darwin or windows", "a.go", "//go:build darwin || windows\n\npackage p\n", true},
		{"windows and not linux", "a.go", "//go:build windows && !linux\n\npackage p\n", false},
		{"plus build line", "a.go", "// +build ignore\n\npackage p\n", false},
		{"name and tag disagree", "a_windows.go", "//go:build linux\n\npackage p\n", false},
		{"begins with underscore", "_a.go", "package p\n", false},
		{"amd64.v1 tag", "a.go", "//go:build amd64.v1\n\npackage p\n", true},
		{"arm64.v8.0 tag", "a.go", "//go:build arm64.v8.0\n\npackage p\n", true},
		{"no default instruction set level", "a.go", "//go:build !amd64.v1 && !arm64.v8.0\n\npackage p\n", false},
		{"a level above the default", "a.go", "//go:build amd64.v3 || arm64.v9.0\n\npackage p\n", false},
		{"a default experiment off", "a.go", "//go:build !goexperiment.regabiargs\n\npackage p\n", false},
	}
	contexts := lintTargetContexts(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, tt.file), []byte(tt.src), 0o644))
			got, err := buildsForLintTarget(contexts, dir, tt.file)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// skippedLocation reports why "go list ./..." and so golangci-lint skip
// the Go file at rel (a slash path below the module root), or "" when
// they do not: a directory or file name that begins with "_" or ".",
// a directory named testdata or vendor, or a directory under another
// go.mod. moduleDirs are the directories of the other go.mod files.
func skippedLocation(rel string, moduleDirs []string) string {
	parts := strings.Split(rel, "/")
	for _, p := range parts {
		switch {
		case strings.HasPrefix(p, "_") || strings.HasPrefix(p, "."):
			return "a path element that begins with _ or ."
		case p == "testdata" || p == "vendor":
			return "a " + p + " directory"
		}
	}
	for _, d := range moduleDirs {
		if strings.HasPrefix(rel, d+"/") {
			return "a directory of its own module (" + d + "/go.mod)"
		}
	}
	return ""
}

// TestSkippedLocation pins skippedLocation.
func TestSkippedLocation(t *testing.T) {
	mods := []string{"sub/mod"}
	tests := []struct {
		name string
		rel  string
		want bool
	}{
		{"file at the root", "main.go", false},
		{"file in a package", "tools/ci/fast.go", false},
		{"nested testdata", "tools/ci/testdata/x.go", true},
		{"testdata at the root", "testdata/x.go", true},
		{"vendor", "vendor/x/y.go", true},
		{"underscore directory", "_x/y.go", true},
		{"dot directory", "tools/.x/y.go", true},
		{"underscore file", "tools/_y.go", true},
		{"nested module", "sub/mod/x.go", true},
		{"sibling of a nested module", "sub/other/x.go", false},
		{"name that extends a nested module", "sub/module/x.go", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, skippedLocation(tt.rel, mods) != "")
		})
	}
}

// TestEveryGoFileIsLinted checks that the linter reaches each Go file
// of the repository, the tracked ones and the ones not yet added: the
// lint step lints each of lintTargets with cgo off (fastSteps), so a
// file must build for one of them, and it must sit where "go list ./..."
// looks (ADR 0007, rule 8). A file that is skipped is a file whose
// findings nobody sees.
func TestEveryGoFileIsLinted(t *testing.T) {
	root := moduleRoot(t)
	var moduleDirs []string
	for _, f := range repoFiles(t, root, "*go.mod") {
		if filepath.Base(f) == "go.mod" && filepath.Dir(f) != "." {
			moduleDirs = append(moduleDirs, filepath.ToSlash(filepath.Dir(f)))
		}
	}
	files := repoFiles(t, root, "*.go")
	require.NotEmpty(t, files, "the Go files of the repository")
	contexts := lintTargetContexts(t)
	for _, f := range files {
		if why := skippedLocation(filepath.ToSlash(f), moduleDirs); why != "" {
			assert.Failf(t, "not linted", "%s is in %s, which go list skips", f, why)
			continue
		}
		ok, err := buildsForLintTarget(contexts, filepath.Join(root, filepath.Dir(f)), filepath.Base(f))
		require.NoError(t, err, "build constraints of %s", f)
		assert.True(t, ok, "%s builds for none of %v with cgo off, so the lint steps skip it", f, lintTargets)
	}
}

// allowedGoModDirectives are the directives go.mod may hold, the ones it
// holds today. Every other one is a decision to review here first: an
// "ignore" takes a directory out of "./...", and so out of the lint and
// the unit steps, and a "replace" or a "tool" changes the code that is
// built.
var allowedGoModDirectives = []string{"module", "go", "require"}

// goModDirectives returns the directives of a go.mod file, each one
// time it starts a line or a block, read as the go command reads the
// file (golang.org/x/mod/modfile, which the go command vendors as
// cmd/vendor/golang.org/x/mod/modfile): a directive is the first word
// of a line, comments start with "//", and a line that ends with "("
// opens a block whose lines are its entries, up to the line ")".
func goModDirectives(data []byte) []string {
	var found []string
	inBlock := false
	for line := range strings.SplitSeq(string(data), "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case inBlock:
			inBlock = line != ")"
		default:
			found = append(found, strings.Fields(line)[0])
			inBlock = strings.HasSuffix(line, "(")
		}
	}
	return found
}

// TestGoModDirectives pins goModDirectives on the forms of a directive.
func TestGoModDirectives(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"line", "module m\n\ngo 1.27.0\n", []string{"module", "go"}},
		{"block", "module m\n\nrequire (\n\ta v1.0.0\n\tb v1.0.0 // indirect\n)\n", []string{"module", "require"}},
		{"ignore line", "module m\n\nignore ./esc\n", []string{"module", "ignore"}},
		{"ignore block", "module m\n\nignore (\n\t./esc\n)\n", []string{"module", "ignore"}},
		{"after a block", "require (\n\ta v1.0.0\n)\nreplace a => ./a\n", []string{"require", "replace"}},
		{"indented, with a comment", "// about it\n  tool x // the tool\n", []string{"tool"}},
		{"CRLF line ends", "module m\r\nignore ./esc\r\n", []string{"module", "ignore"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, goModDirectives([]byte(tt.src)))
		})
	}
}

// TestOnlyAllowedGoModDirectives checks that go.mod holds no directive
// outside allowedGoModDirectives: an "ignore" there takes a directory
// out of "./...", and the linter never reads it.
func TestOnlyAllowedGoModDirectives(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(moduleRoot(t), "go.mod"))
	require.NoError(t, err)
	found := goModDirectives(data)
	require.NotEmpty(t, found, "the directives of go.mod")
	for _, name := range found {
		assert.Contains(t, allowedGoModDirectives, name, "go.mod holds a directive that is not allowed")
	}
}

// pinnedTools are the tools the mise configuration of this repository
// pins, each at the version mise.lock locks: the go command and
// golangci-lint, the two that fast runs (resolveLintTools), govulncheck,
// which all starts through "mise exec" (allSteps), and gh, for the
// release tool (decision DR7 of review round 8: gh is locked, and the
// tool starts it by its resolved path). This is the one list of what
// mise may install, and the one that 12 12.1 of the specification
// points at: allowedMiseTables, TestLockedVersions and TestMiseConfigs
// read it.
var pinnedTools = []string{"go", "gh", "golangci-lint", "go:golang.org/x/vuln/cmd/govulncheck"}

// allowedMiseTables are the tables a mise configuration file of this
// repository may hold, with the keys each may hold. The [env] table,
// the "env_file" setting, [tasks] and [hooks] are not on the list: each
// one adds to the environment of what mise starts, or starts a program
// of its own. [tools] holds the pinnedTools alone, each with the
// version mise.lock locks (miseFindings), so no tool is named by a path
// or by a version that the lock does not hold.
var allowedMiseTables = map[string][]string{
	"settings": {"lockfile"},
	"tools":    pinnedTools,
}

// bareKey matches a TOML key that needs no quotes
// (https://toml.io/en/v1.0.0#keys).
var bareKey = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// tomlKey writes a tool name as a key of mise.toml: bare when TOML
// allows it, quoted otherwise, as a backend name with ":" or "/" is.
func tomlKey(name string) string {
	if bareKey.MatchString(name) {
		return name
	}
	return `"` + name + `"`
}

// lockedVersion matches a version as mise.lock holds one: exact, three
// numbers.
var lockedVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// TestLockedVersions checks that mise.lock locks the pinnedTools and no
// other, each at an exact version.
func TestLockedVersions(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(moduleRoot(t), "mise.lock"))
	require.NoError(t, err)
	locked, err := lockedVersions(data)
	require.NoError(t, err)
	assert.ElementsMatch(t, pinnedTools, slices.Collect(maps.Keys(locked)), "the tools of mise.lock")
	for tool, version := range locked {
		assert.Regexp(t, lockedVersion, version, "the version of %s in mise.lock", tool)
	}
}

// miseFindings returns what a mise configuration file holds outside
// allowedMiseTables: a key before the first table, a table that is not
// on the list, a key of a table that is not on its list, a tool whose
// value is not the version that locked holds for it, as "1.27.0" with
// its quotes, and a line that is neither a table nor a "key = value"
// line on one line, as a value that spans lines is.
func miseFindings(data []byte, locked map[string]string) []string {
	var bad []string
	table, inTable := "", false
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			table = strings.TrimSpace(strings.Trim(line, "[]"))
			_, inTable = allowedMiseTables[table]
			if !inTable {
				bad = append(bad, "table ["+table+"]")
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.Trim(strings.TrimSpace(key), `"'`)
		switch {
		case !ok:
			bad = append(bad, "line "+line)
		case table == "":
			bad = append(bad, "key "+key+" before the first table")
		case inTable && !slices.Contains(allowedMiseTables[table], key):
			bad = append(bad, "key "+key+" in ["+table+"]")
		case table == "tools" && strings.TrimSpace(value) != `"`+locked[key]+`"`:
			bad = append(bad, "value of "+key+" in [tools]")
		}
	}
	return bad
}

// TestMiseFindings pins miseFindings on the forms that add to the
// environment, on the forms that name a tool mise did not install from
// the lock, and on the configuration of this repository.
func TestMiseFindings(t *testing.T) {
	locked := map[string]string{"go": "1.27.0", "golangci-lint": "2.14.0"}
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"the repository's", "# about\n[settings]\nlockfile = true\n\n[tools]\ngo = \"1.27.0\"\ngolangci-lint = \"2.14.0\"\n", nil},
		{"env table", "[env]\nGOFLAGS = \"-tags=hide\"\n", []string{"table [env]"}},
		{"env subtable", "[env._]\nfile = \".env\"\n", []string{"table [env._]"}},
		{"env array of tables", "[[env]]\nGOFLAGS = \"x\"\n", []string{"table [env]"}},
		{"dotted env key at the top", "env.GOFLAGS = \"x\"\n", []string{"key env.GOFLAGS before the first table"}},
		{"env file setting", "[settings]\nenv_file = \".env\"\n", []string{"key env_file in [settings]"}},
		{"tasks", "[tasks.lint]\nrun = \"true\"\n", []string{"table [tasks.lint]"}},
		{"value over lines", "[tools]\ngo = [\n\"1\",\n]\n", []string{"value of go in [tools]", "line \"1\",", "line ]"}},
		{"a tool by a path", "[tools]\ngolangci-lint = \"path:./.lint\"\n", []string{"value of golangci-lint in [tools]"}},
		{"a version the lock does not hold", "[tools]\ngo = \"1.26.0\"\n", []string{"value of go in [tools]"}},
		{"a version prefix", "[tools]\ngo = \"1.27\"\n", []string{"value of go in [tools]"}},
		{"an inline table", "[tools]\ngo = { version = \"1.27.0\" }\n", []string{"value of go in [tools]"}},
		{"a quoted key", "[tools]\n\"golangci-lint\" = \"2.14.0\"\n", nil},
		{"a tool that is not pinned", "[tools]\nnode = \"24.0.0\"\n", []string{"key node in [tools]"}},
		{"a tool subtable", "[tools.golangci-lint]\nversion = \"2.14.0\"\n", []string{"table [tools.golangci-lint]"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, miseFindings([]byte(tt.src), locked))
		})
	}
}

// TestMiseConfigs checks every mise configuration file of the
// repository, tracked or not (mise.toml, mise.local.toml, .mise.toml,
// .config/mise and the others whose path names mise), against
// allowedMiseTables and the versions of mise.lock, and that mise.toml
// pins each of pinnedTools. fast starts no tool through mise, so an
// [env] table cannot reach a step; this keeps one out of "go vet" and
// "go test" too, when a developer starts them through mise. No
// .tool-versions file is allowed, since mise reads one as a
// configuration of tools too.
func TestMiseConfigs(t *testing.T) {
	root := moduleRoot(t)
	lock, err := os.ReadFile(filepath.Join(root, "mise.lock"))
	require.NoError(t, err)
	locked, err := lockedVersions(lock)
	require.NoError(t, err)
	var configs []string
	for _, f := range repoFiles(t, root, "*mise*") {
		if strings.HasSuffix(f, ".toml") {
			configs = append(configs, f)
		}
	}
	require.Contains(t, configs, "mise.toml")
	for _, f := range configs {
		data, err := os.ReadFile(filepath.Join(root, f))
		require.NoError(t, err)
		assert.Empty(t, miseFindings(data, locked), "%s holds what is not on allowedMiseTables", f)
		if f == "mise.toml" {
			lines := strings.Split(string(data), "\n")
			for _, tool := range pinnedTools {
				assert.Contains(t, lines, tomlKey(tool)+` = "`+locked[tool]+`"`, "mise.toml pins %s", tool)
			}
		}
	}
	assert.Empty(t, repoFiles(t, root, "*.tool-versions"), "the .tool-versions files of the repository")
}

// TestGoDirectiveEqualsMisePin checks that the go directive of go.mod
// is the go version mise.toml pins, so the toolchain the checks run
// and the one the module declares cannot drift apart.
func TestGoDirectiveEqualsMisePin(t *testing.T) {
	root := moduleRoot(t)
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	require.NoError(t, err)
	toml, err := os.ReadFile(filepath.Join(root, "mise.toml"))
	require.NoError(t, err)
	directive, ok := goDirective(mod)
	require.True(t, ok, "go.mod holds a go directive")
	pin, ok := misePin(toml, "go")
	require.True(t, ok, "mise.toml pins go")
	assert.Equal(t, pin, directive, "the go directive of go.mod and the go pin of mise.toml")
}

// goDirective returns the version of the go directive of a go.mod.
func goDirective(data []byte) (string, bool) {
	for line := range strings.SplitSeq(string(data), "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		if f := strings.Fields(line); len(f) == 2 && f[0] == "go" {
			return f[1], true
		}
	}
	return "", false
}

// misePin returns the version a mise.toml pins for tool in [tools]. It
// reads the one-line forms miseFindings allows and nothing else, and
// TestMiseConfigs runs miseFindings on the same file.
func misePin(data []byte, tool string) (string, bool) {
	inTools := false
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			inTools = line == "[tools]"
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if inTools && ok && strings.Trim(strings.TrimSpace(key), `"'`) == tool {
			return strings.Trim(strings.TrimSpace(value), `"`), true
		}
	}
	return "", false
}

// TestGoDirectiveParsers pins goDirective and misePin on the forms they
// must read and on the ones that must not match.
func TestGoDirectiveParsers(t *testing.T) {
	v, ok := goDirective([]byte("module x\n\ngo 1.27.2 // note\n"))
	assert.True(t, ok)
	assert.Equal(t, "1.27.2", v)
	_, ok = goDirective([]byte("module x\n// go 1.27.2\n"))
	assert.False(t, ok, "a comment is no directive")
	_, ok = goDirective([]byte("module x\ntoolchain go1.27.2\n"))
	assert.False(t, ok, "toolchain is no go directive")
	v, ok = misePin([]byte("[settings]\ngo = \"0.0.0\"\n[tools]\ngo = \"1.27.2\"\n"), "go")
	assert.True(t, ok)
	assert.Equal(t, "1.27.2", v)
	v, ok = misePin([]byte("[tools]\n\"go\" = \"1.27.2\"\n"), "go")
	assert.True(t, ok, "a quoted key")
	assert.Equal(t, "1.27.2", v)
	_, ok = misePin([]byte("[settings]\ngo = \"1.27.2\"\n"), "go")
	assert.False(t, ok, "a go key outside [tools] is no pin")
}
