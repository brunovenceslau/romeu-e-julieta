// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package egress_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/egress"
)

// TestShapeIsTheMiseOne: shape.go is internal/mise/scan.go under
// unexported names (10 10.7 forbids the import), and this holds the copy
// to the original, so the limits that run before the decoder cannot
// drift apart.
func TestShapeIsTheMiseOne(t *testing.T) {
	theirs, err := os.ReadFile("../mise/scan.go")
	require.NoError(t, err)
	ours, err := os.ReadFile("shape.go")
	require.NoError(t, err)
	from := func(s, marker string) string {
		_, rest, ok := strings.Cut(s, marker)
		require.True(t, ok, marker)
		return marker + rest
	}
	want := strings.NewReplacer("MaxDepth", "maxDepth", "MaxKeyParts", "maxKeyParts", "CheckShape", "checkShape").
		Replace(from(string(theirs), "// MaxDepth is"))
	assert.Equal(t, want, from(string(ours), "// maxDepth is"))
}

// TestParseLockShapeSkipsStrings: brackets and dots inside a string or a
// comment count for neither limit, in each of the four string forms, and
// one outside them still does.
func TestParseLockShapeSkipsStrings(t *testing.T) {
	deep := strings.Repeat("[", 40) + strings.Repeat(".", 20)
	head := "lockfile_version = 3\n[[tools.gh]]\nbackend = \"aqua:cli/cli\"\n"
	accepted := []struct{ name, data string }{
		{"a basic string", "version = \"" + deep + "\\\"" + deep + "\"\n"},
		{"a literal string", "version = '" + deep + "'\n"},
		{"a multi-line basic string", "version = \"\"\"\n" + deep + "\\\"\"\"\n" + deep + "\"\"\"\n"},
		{"a multi-line literal string", "version = '''\n" + deep + "\n'''\n"},
		{"a comment", "# " + deep + "\n"},
		{"a quoted key", "\"a.b.c.d.e.f.g.h.i\" = 1\n"},
	}
	for _, tt := range accepted {
		t.Run(strings.ReplaceAll(tt.name, " ", "_"), func(t *testing.T) {
			_, err := egress.ParseLock([]byte(head + tt.data))
			require.NoError(t, err)
		})
	}
	refused := []struct{ name, data, want string }{
		{"an unclosed string ends at the line", "version = \"x\n" + strings.Repeat("[", 40) + "\n", "nesting"},
		{"an unclosed multi-line string ends the data", "version = \"\"\"" + deep, "mise.lock"},
		{"an unclosed literal string ends at the line", "version = 'x\n" + strings.Repeat("a.", 9) + "b = 1\n", "key parts"},
	}
	for _, tt := range refused {
		t.Run(strings.ReplaceAll(tt.name, " ", "_"), func(t *testing.T) {
			_, err := egress.ParseLock([]byte(head + tt.data))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}
