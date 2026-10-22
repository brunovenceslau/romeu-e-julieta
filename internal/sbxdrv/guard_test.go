// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
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

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// call is a Call of the tests with the argv given.
func call(argv ...string) Call {
	return Call{Name: "test", Argv: argv, Shape: argv, Class: procgroup.SbxRead, Mode: Captured}
}

// TestGuardRefusesApproval holds I4: whatever the call, --auto-approve
// and -y; on env run and env create, the --force and -f that sbx v0.48.0
// put in their place.
func TestGuardRefusesApproval(t *testing.T) {
	tests := []struct {
		name string
		argv []string
	}{
		{"auto-approve", []string{"env", "run", "-d", "--auto-approve", "/d"}},
		{"auto-approve with a value", []string{"env", "run", "--auto-approve=true", "/d"}},
		{"auto-approve on another call", []string{"stop", "--auto-approve", "shop"}},
		{"-y", []string{"env", "run", "-y", "/d"}},
		{"-y in a cluster", []string{"env", "run", "-dy", "/d"}},
		{"-y on rm", []string{"env", "rm", "-y", "/d"}},
		{"force on run", []string{"env", "run", "-d", "--force", "/d"}},
		{"force on create", []string{"env", "create", "--force", "/d"}},
		{"-f on run", []string{"env", "run", "-f", "/d"}},
		{"-f in a cluster on run", []string{"env", "run", "-df", "/d"}},
		{"-f after an env value", []string{"env", "run", "--env-arg", "x=1", "-f", "/d"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, Guard(call(tt.argv...)))
		})
	}
}

// TestGuardLeavesWhatItDoesNotGuard holds the other side: the words
// of a command after "--", the value of a flag, --force on env rm, and
// the calls the table builds.
func TestGuardLeavesWhatItDoesNotGuard(t *testing.T) {
	tests := []struct {
		name string
		argv []string
	}{
		{"a command after the double dash", []string{"env", "exec", "/d", "--", "tool", "--auto-approve", "-y", "--force"}},
		{"the value of --env", []string{"env", "exec", "-it", "--env", "-y", "/d", "--", "sh"}},
		{"the value of --command", []string{"secret", "set", "github", "--command", "-f"}},
		{"force on rm", []string{"env", "rm", "--force", "/d"}},
		{"a cluster for a terminal", []string{"env", "exec", "-it", "/d", "--", "sh"}},
		{"detach", []string{"env", "run", "-d", "--clone", "/d"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, Guard(call(tt.argv...)))
		})
	}
}

// TestGuardRefusesSecretValueForms holds I25: sbx secret set only with
// --command, and with no other flag than the three of secretSetFlags.
func TestGuardRefusesSecretValueForms(t *testing.T) {
	tests := []struct {
		name string
		argv []string
	}{
		{"no source", []string{"secret", "set", "github"}},
		{"-t", []string{"secret", "set", "github", "-t", "ghp_x"}},
		{"-t joined", []string{"secret", "set", "github", "-tghp_x"}},
		{"--token", []string{"secret", "set", "github", "--token", "ghp_x"}},
		{"--token=", []string{"secret", "set", "github", "--token=ghp_x"}},
		{"--token with a command", []string{"secret", "set", "github", "--command", "/bin/true", "--token", "x"}},
		{"stdin", []string{"secret", "set", "--registry", "ghcr.io", "--password-stdin"}},
		{"oauth", []string{"secret", "set", "openai", "--oauth"}},
		{"reference", []string{"secret", "set", "anthropic", "--ref", "op://a/b/c"}},
		{"force", []string{"secret", "set", "github", "--command", "/bin/true", "--force"}},
		{"-f", []string{"secret", "set", "github", "--command", "/bin/true", "-f"}},
		{"an empty command", []string{"secret", "set", "github", "--command", ""}},
		{"a command at the end of the argv", []string{"secret", "set", "github", "--command"}},
		{"an empty command with =", []string{"secret", "set", "github", "--command="}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, Guard(call(tt.argv...)))
		})
	}
	assert.NoError(t, Guard(call("secret", "set", "github", "--command", "/usr/bin/true", "--sandbox", "shop", "--refresh", "55m")))
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
