// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lint

import (
	"strings"
)

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

// testifylintFixtures holds one violation of each testifylint checker
// of testifyViolations, one per file.
func testifylintFixtures() []lintFixture {
	var fixtures []lintFixture
	for _, v := range testifyViolations {
		fixtures = append(fixtures, lintFixture{
			file:    strings.ReplaceAll(v.checker, "-", "") + "_test.go",
			source:  v.source(),
			linter:  "testifylint",
			message: v.checker + ": ",
		})
	}
	return fixtures
}
