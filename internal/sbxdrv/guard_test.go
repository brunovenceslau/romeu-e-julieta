// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"bytes"
	jsonv2 "encoding/json/v2"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// call is a Call of the tests with the argv given.
func call(argv ...string) Call {
	return Call{Name: "test", Argv: argv, Shape: argv, Class: procgroup.SbxRead, Mode: Captured}
}

// guardCase is one row of the table both guards are held to
// (e2e/testdata/sbx/guard-cases.jsonl): the argv, and what the guard of
// this package says of it. Want is I4 or I25 when both this guard and
// the fake sbx refuse it (the fake exits 99), "shape" when this guard
// refuses a call that is not one of the shapes romeu drives and the fake
// has no rule for it, and "ok" when this guard lets it through and the
// fake does not exit 99 on it.
type guardCase struct {
	Want string   `json:"want"`
	Argv []string `json:"argv"`
	Note string   `json:"note"`
}

// guardCasesPath is the table, from this package.
const guardCasesPath = "../../e2e/testdata/sbx/guard-cases.jsonl"

// loadGuardCases reads the table.
func loadGuardCases(t *testing.T) []guardCase {
	t.Helper()
	data, err := os.ReadFile(guardCasesPath)
	require.NoError(t, err)
	var out []guardCase
	for i, line := range slices.Collect(bytes.Lines(data)) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var c guardCase
		require.NoError(t, jsonv2.Unmarshal(line, &c), "line %d", i+1)
		require.Contains(t, []string{"I4", "I25", "shape", "ok"}, c.Want, "line %d", i+1)
		out = append(out, c)
	}
	require.NotEmpty(t, out)
	return out
}

// invariantOf returns the invariant a guard error names.
func invariantOf(err error) string {
	for _, d := range cli.Of(err).Details {
		if d.Key == "invariant" {
			return d.Value
		}
	}
	return ""
}

// TestGuardCases holds the guard to the shared table: I4 wherever an
// approval is, found after the global flags and for the flags of the
// verb it is on (the -t of env exec is a boolean, the -t of secret set a
// value); I25 for every form of secret set but the exact one; a shape
// romeu does not drive is refused; the rest passes.
func TestGuardCases(t *testing.T) {
	for _, tt := range loadGuardCases(t) {
		t.Run(tt.Want+" "+strings.Join(tt.Argv, " "), func(t *testing.T) {
			err := Guard(call(tt.Argv...))
			if tt.Want == "ok" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, tt.Want, invariantOf(err), err.Error())
		})
	}
}

// TestGuardTableHasTheRowsThatKilledMutants holds the table to the rows
// a review found surviving: the fake sbx with --token among its secret
// flags, and the fake with create dropped from the force rule.
func TestGuardTableHasTheRowsThatKilledMutants(t *testing.T) {
	cases := loadGuardCases(t)
	has := func(want string, argv ...string) bool {
		return slices.ContainsFunc(cases, func(c guardCase) bool { return c.Want == want && slices.Equal(c.Argv, argv) })
	}
	assert.True(t, has("I25", "secret", "set", "github", "--command", "/bin/true", "--token", "x"))
	assert.True(t, has("I25", "secret", "set", "github", "--token", "ghp_x"))
	assert.True(t, has("I4", "env", "create", "--force", "/abs/shop-env"))
}

// forbiddenWords are the words no builder source may hold outside
// guard.go: the flags I4 refuses.
var forbiddenWords = []string{"--auto-approve", "-y", "--force", "-f", "--token", "-t", "--password-stdin", "--oauth", "--ref"}

// literalFindings returns, for the Go source src of the file name, the
// string literals that are one of the words: the scan of I4 and I25
// over the builders.
func literalFindings(t *testing.T, name string, src []byte, words []string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, src, parser.SkipObjectResolution)
	require.NoError(t, err)
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if s, err := strconv.Unquote(lit.Value); err == nil && slices.Contains(words, s) {
			out = append(out, name+": "+lit.Value)
		}
		return true
	})
	return out
}

// TestNoBuilderEmitsAForbiddenFlag is the scan of I4 over the builders:
// no source file of the package but the guard holds one of the flags
// the guard refuses. The one exception is env rm, which may say
// --force, and is named in its own file.
func TestNoBuilderEmitsAForbiddenFlag(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || f == "guard.go" {
			continue
		}
		src, err := os.ReadFile(f)
		require.NoError(t, err)
		words := forbiddenWords
		if f == "envrm.go" {
			words = slices.DeleteFunc(slices.Clone(words), func(w string) bool { return w == "--force" })
		}
		assert.Empty(t, literalFindings(t, f, src, words), f)
	}
}

// TestScanFindsAPlantedFlag is the positive control of the scan: a
// builder that says --auto-approve is found.
func TestScanFindsAPlantedFlag(t *testing.T) {
	planted := []byte("package x\n\nfunc b() []string { return []string{\"env\", \"run\", \"--auto-approve\"} }\n")
	assert.Len(t, literalFindings(t, "planted.go", planted, forbiddenWords), 1)
	clean := []byte("package x\n\nfunc b() []string { return []string{\"env\", \"run\", \"-d\"} }\n")
	assert.Empty(t, literalFindings(t, "clean.go", clean, forbiddenWords))
}
