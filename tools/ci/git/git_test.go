// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package git_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

func TestRun(t *testing.T) {
	r := gittest.New(t)
	r.Write("a.txt", "a\n")
	sha := r.Commit("a")

	tests := []struct {
		name    string
		stdin   string
		args    []string
		want    string
		wantErr string
	}{
		{"standard output is returned", "", []string{"rev-parse", "HEAD"}, sha + "\n", ""},
		{"standard input is passed", sha + "\n", []string{"cat-file", "--batch-check"}, sha + " commit ", ""},
		{"a failing command carries what git said", "", []string{"rev-parse", "--verify", "nonesuch"}, "", "git rev-parse:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := r.Run(t.Context(), []byte(tt.stdin), tt.args...)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Contains(t, err.Error(), "fatal", "git's own message")
				return
			}
			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(string(out), tt.want), "output %q, want prefix %q", out, tt.want)
		})
	}
}

// TestRunIgnoresReplaceRefs shows that a replace ref does not change
// what Run reads: a push sends the objects themselves, so the checks
// read those and not a local stand-in.
func TestRunIgnoresReplaceRefs(t *testing.T) {
	r := gittest.New(t)
	r.Write("a.txt", "a\n")
	base := r.Commit("base")
	r.Write("a.txt", "the real line\n")
	real := r.Commit("real")
	r.Git("checkout", "--quiet", "--detach", base)
	r.Write("a.txt", "a stand-in line\n")
	standIn := r.Commit("stand-in")
	r.Git("replace", real, standIn)
	out, err := r.Run(t.Context(), nil, "cat-file", "-p", real+":a.txt")
	require.NoError(t, err)
	assert.Equal(t, "the real line\n", string(out), "read through a replace ref, want the real object")
}

// TestRunPrintableError shows that what git says on failure is kept,
// with no control character of a path it repeats. Git 2.53 already
// replaces those in its own fatal messages; Run quotes what still
// holds one, for a git or a message that does not.
func TestRunPrintableError(t *testing.T) {
	r := gittest.New(t)
	r.Write("a.txt", "a\n")
	r.Commit("a")
	_, err := r.Run(t.Context(), nil, "cat-file", "-p", "HEAD:x\x1b[2Jy")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "\x1b", "the escape of the path")
	assert.Contains(t, err.Error(), "fatal", "git's message")
}

func TestPrintable(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"plain ASCII", "docs/a b.md", "docs/a b.md"},
		{"plain outside ASCII", "docs/caf\u00e9.md", "docs/caf\u00e9.md"},
		{"an escape", "a\x1bb", `"a\x1bb"`},
		{"a newline", "a\nb", `"a\nb"`},
		{"a tab", "a\tb", `"a\tb"`},
		{"DEL", "a\x7fb", `"a\x7fb"`},
		{"a zero-width space", "a\u200bb", `"a\u200bb"`},
		{"not UTF-8", "a\xffb", `"a\xffb"`},
		{"quoted keeps only ASCII", "caf\u00e9\n", `"caf\u00e9\n"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, git.Printable(tt.in))
		})
	}
}

func TestSafeLines(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"plain text and a tab stay", "a\tb\nplain", "a\tb\nplain"},
		{"an escape and a CR", "x\x1b[2Jy\r", `x\x1b[2Jy\r`},
		{"not UTF-8", "a\xffb", `a\xffb`},
		{"a line separator inside a line", "a\u2028b", `a\u2028b`},
		{"a right-to-left override inside a line", "a\u202eb", `a\u202eb`},
		{"a command marker", "::error::x", "./::error::x"},
		{"a command marker after spaces", "  ::error::x", "./  ::error::x"},
		{"a command marker after a tab", "\t::error::x/f", "./\t::error::x/f"},
		{"a no-break space before a marker is escaped", "d\n\u00a0::error::x", `d` + "\n" + `\u00a0::error::x`},
		{"an em space before a marker is escaped", "\u2003::error::x", `\u2003::error::x`},
		{"a zero-width space before a marker is escaped", "\u200b::error::x", `\u200b::error::x`},
		{"a BOM before a marker is escaped", "\ufeff::error::x", `\ufeff::error::x`},
		{"an Azure marker", "##[error]x", `##\[error]x`},
		{"an Azure marker in the middle of a line", "x ##[error]y", `x ##\[error]y`},
		{"a repeated hash before the bracket", "###[error]", `###\[error]`},
		{"a marker in the middle is text", "a ::error::x", "a ::error::x"},
		{"a second line after a separator", "d\n\u00a0::error::forged\u2028::error::two", "d\n" + `\u00a0::error::forged\u2028::error::two`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, git.SafeLines(tt.in))
		})
	}
}

// TestSafeLinesIgnorableRunes checks that a rune that prints but that a
// culture-sensitive comparison may skip (a combining mark, a variation
// selector, a filler) cannot hide a marker: in the output, the line
// stripped to printable ASCII neither starts with "::" nor holds "##[".
func TestSafeLinesIgnorableRunes(t *testing.T) {
	skeleton := func(line string) string {
		return strings.Map(func(r rune) rune {
			if r == '\t' || (r >= 0x20 && r <= 0x7e) {
				return r
			}
			return -1
		}, line)
	}
	for _, r := range []rune{0x034f, 0xfe0f, 0xe0100, 0x180b, 0x2800, 0x3164, 0x115f, 0x1160, 0xffa0, 0x17b4, 0x0300, 0x2d7f} {
		c := string(r)
		for _, in := range []string{
			c + "::error::x",
			" \t" + c + " ::error::x",
			":" + c + ":error::x",
			"#" + c + "#[error]x",
			"##" + c + "[error]x",
			"x ##" + c + "[error]x",
			"x #" + c + "#[error]x",
		} {
			out := git.SafeLines(in)
			sk := skeleton(out)
			assert.False(t, strings.HasPrefix(strings.TrimLeft(sk, " \t"), "::"), "%U %q -> %q", r, in, out)
			assert.NotContains(t, sk, "##[", "%U %q -> %q", r, in, out)
		}
	}
	assert.Equal(t, "./\u034f::error::x", git.SafeLines("\u034f::error::x"))
	assert.Equal(t, `x #\u034f#[error]`, git.SafeLines("x #\u034f#[error]"))
}

func TestRunStopsWithItsContext(t *testing.T) {
	r := gittest.New(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := r.Run(ctx, nil, "rev-parse", "--git-dir")
	require.Error(t, err, "Run under a cancelled context")
}
