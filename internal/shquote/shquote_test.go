// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package shquote_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/shquote"
)

// The table of the shell quoter (10 10.1, REQ-0758): each word and the
// one form Quote writes for it.
func TestQuote(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"an empty word is two quotes", "", "''"},
		{"letters and digits stay bare", "abc123", "abc123"},
		{"an absolute path stays bare", "/usr/bin/security", "/usr/bin/security"},
		{"a flag stays bare", "-w", "-w"},
		{"the safe punctuation stays bare", "a_b-c.d/e:f,g+h@i%j", "a_b-c.d/e:f,g+h@i%j"},
		{"a space is quoted", "a b", "'a b'"},
		{"a single quote is closed, escaped and reopened", "it's", `'it'\''s'`},
		{"a word of one single quote", "'", `''\'''`},
		{"a dollar is quoted", "$HOME", "'$HOME'"},
		{"a command substitution is quoted", "$(id)", "'$(id)'"},
		{"a backquote is quoted", "`id`", "'`id`'"},
		{"a backslash is quoted", `a\b`, `'a\b'`},
		{"a double quote is quoted", `a"b`, `'a"b'`},
		{"a glob is quoted", "*.go", "'*.go'"},
		{"a question mark is quoted", "a?", "'a?'"},
		{"brackets are quoted", "[a]", "'[a]'"},
		{"a leading tilde is quoted", "~root", "'~root'"},
		{"a leading hash is quoted", "#x", "'#x'"},
		{"an equals sign is quoted, so no word reads as an assignment", "A=b", "'A=b'"},
		{"a semicolon is quoted", "a;b", "'a;b'"},
		{"a pipe and an ampersand are quoted", "a|b&c", "'a|b&c'"},
		{"redirections are quoted", "<a>b", "'<a>b'"},
		{"parentheses and braces are quoted", "(a){b}", "'(a){b}'"},
		{"a newline is quoted", "a\nb", "'a\nb'"},
		{"a tab is quoted", "a\tb", "'a\tb'"},
		{"an exclamation mark is quoted", "a!", "'a!'"},
		{"a caret is quoted", "a^b", "'a^b'"},
		{"text that is not ASCII is quoted", "café", "'café'"},
		{"a 1Password reference stays bare", "op://dev/npm/token", "op://dev/npm/token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := shquote.Quote(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, []string{tt.in}, split(t, got), "the word reads back as itself")
		})
	}
}

// A NUL byte cannot be carried by any word of sh, so Quote refuses it
// rather than write a word that reads back as another.
func TestQuoteRefusesNUL(t *testing.T) {
	_, err := shquote.Quote("a\x00b")
	require.ErrorIs(t, err, shquote.ErrNUL)
}

// Join quotes each word and joins them with one space.
func TestJoin(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"no word is the empty line", nil, ""},
		{"one word", []string{"/bin/true"}, "/bin/true"},
		{"a keychain read", []string{"/usr/bin/security", "find-generic-password", "-s", "romeu/shop/github", "-w"},
			"/usr/bin/security find-generic-password -s romeu/shop/github -w"},
		{"an empty word in the middle", []string{"a", "", "b"}, "a '' b"},
		{"a word with a space", []string{"/opt/x y/op", "read"}, "'/opt/x y/op' read"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := shquote.Join(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			if len(tt.in) > 0 {
				assert.Equal(t, tt.in, split(t, got))
			}
		})
	}
}

// Join refuses a word with a NUL byte, and names its place.
func TestJoinRefusesNUL(t *testing.T) {
	_, err := shquote.Join([]string{"a", "b\x00"})
	require.ErrorIs(t, err, shquote.ErrNUL)
	assert.Contains(t, err.Error(), "word 2")
}

// FuzzQuote checks that any word without a NUL byte reads back as
// itself, and that a bare word holds only the safe characters.
func FuzzQuote(f *testing.F) {
	for _, s := range []string{"", "a", "'", "a b", "$(x)", "''", "\\'", "é"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got, err := shquote.Quote(s)
		if strings.ContainsRune(s, 0) {
			require.ErrorIs(t, err, shquote.ErrNUL)
			return
		}
		require.NoError(t, err)
		assert.Equal(t, []string{s}, split(t, got))
	})
}

// split reads line as sh splits the words Join writes: bare runs of
// characters, single-quoted runs, and a backslash that escapes the next
// byte outside quotes, words being separated by one space. It is the
// reader of the subset Quote writes, not a shell; a form outside that
// subset fails the test.
func split(t *testing.T, line string) []string {
	t.Helper()
	var words []string
	var cur strings.Builder
	in, quoted := false, false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quoted:
			if c == '\'' {
				quoted = false
			} else {
				cur.WriteByte(c)
			}
		case c == '\'':
			quoted, in = true, true
		case c == '\\':
			require.Less(t, i+1, len(line), "a trailing backslash")
			i++
			cur.WriteByte(line[i])
			in = true
		case c == ' ':
			require.True(t, in, "two spaces or a leading space in %q", line)
			words = append(words, cur.String())
			cur.Reset()
			in = false
		default:
			require.Contains(t, safe, string(c), "an unquoted byte that sh reads as syntax in %q", line)
			cur.WriteByte(c)
			in = true
		}
	}
	require.False(t, quoted, "an unclosed quote in %q", line)
	require.True(t, in, "a trailing space in %q", line)
	return append(words, cur.String())
}

// safe are the bytes a bare word may hold.
const safe = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-./:,+@%"
