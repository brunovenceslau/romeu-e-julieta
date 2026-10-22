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

// TestCheckDepth counts the nesting of brackets and braces outside
// strings and comments, and refuses what passes MaxDepth.
func TestCheckDepth(t *testing.T) {
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
			err := mise.CheckDepth([]byte(tt.src))
			if tt.ok {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "nesting")
		})
	}
}
