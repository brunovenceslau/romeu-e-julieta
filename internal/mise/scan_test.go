// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/mise"
)

// TestCheckShape counts the nesting of brackets and braces outside
// strings and comments, and refuses what passes MaxDepth.
func TestCheckShape(t *testing.T) {
	deep := func(n int) string { return "x = " + strings.Repeat("[", n) + strings.Repeat("]", n) + "\n" }
	tests := []struct {
		name string
		src  string
		ok   bool
	}{
		{"flat", "a = 1\n[t]\nb = [1, 2]\n", true},
		{"table headers", "[[tools.a]]\nversion = \"1\"\n[tools.a.\"platforms.x\"]\nc = 1\n", true},
		{"at the cap", deep(mise.MaxDepth), true},
		{"one past the cap", deep(mise.MaxDepth + 1), false},
		{"inline tables", "x = " + strings.Repeat("{a=", mise.MaxDepth+1) + "1" + strings.Repeat("}", mise.MaxDepth+1) + "\n", false},
		{"brackets in a basic string", "x = \"" + strings.Repeat("[", 100) + "\"\n", true},
		{"brackets in a literal string", "x = '" + strings.Repeat("{", 100) + "'\n", true},
		{"an escaped quote in a basic string", "x = \"\\\"" + strings.Repeat("[", 100) + "\"\n", true},
		{"brackets in a comment", "# " + strings.Repeat("[", 100) + "\na = 1\n", true},
		{"brackets in a multi-line basic string", "x = \"\"\"\n" + strings.Repeat("[", 100) + "\n\"\"\"\n", true},
		{"brackets in a multi-line literal string", "x = '''\n" + strings.Repeat("{", 100) + "\n'''\n", true},
		{"a string ends, then nesting", "x = \"a\" # c\ny = " + strings.Repeat("[", mise.MaxDepth+1) + "\n", false},
		{"an empty string then brackets", "x = \"\"\ny = " + strings.Repeat("[", mise.MaxDepth+1) + "\n", false},
		{"closers below zero", strings.Repeat("]", 100) + "\n", true},
		{"unclosed", "x = [", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mise.CheckShape([]byte(tt.src))
			if tt.ok {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "nesting")
		})
	}
}

// TestCheckShapeKeyParts counts the parts of a key by its dots outside
// strings and comments, in a key-value line, a table header and an
// inline table, and refuses what passes MaxKeyParts.
func TestCheckShapeKeyParts(t *testing.T) {
	parts := func(n int) string { return strings.TrimSuffix(strings.Repeat("a.", n), ".") }
	tests := []struct {
		name string
		src  string
		ok   bool
	}{
		{"the keys mise writes", "[[tools.gh]]\n[tools.gh.\"platforms.linux-x64\"]\nchecksum = \"sha256:1.2\"\n", true},
		{"a dotted key at the cap", parts(mise.MaxKeyParts) + " = 1\n", true},
		{"a dotted key past the cap", parts(mise.MaxKeyParts+1) + " = 1\n", false},
		{"a header past the cap", "[" + parts(mise.MaxKeyParts+1) + "]\n", false},
		{"a table array past the cap", "[[" + parts(mise.MaxKeyParts+1) + "]]\n", false},
		{"an inline table key past the cap", "x = {" + parts(mise.MaxKeyParts+1) + " = 1}\n", false},
		{"dots in a quoted key", "\"" + parts(100) + "\" = 1\n", true},
		{"dots in a literal key", "'" + parts(100) + "' = 1\n", true},
		{"dots in a comment", "# " + parts(100) + "\n", true},
		{"dots in a value string", "x = \"" + parts(100) + "\"\n", true},
		{"floats in an array over lines", "x = [\n" + strings.Repeat("1.5,\n", 100) + "]\n", true},
		{"floats on one line", "x = [" + strings.Repeat("1.5, ", 100) + "]\n", true},
		{"a count that restarts at each line", strings.Repeat(parts(mise.MaxKeyParts)+" = 1\n", 100), true},
		{"quoted parts and bare ones", parts(mise.MaxKeyParts-1) + ".\"" + parts(100) + "\" = 1\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mise.CheckShape([]byte(tt.src))
			if tt.ok {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "key parts")
		})
	}
}
