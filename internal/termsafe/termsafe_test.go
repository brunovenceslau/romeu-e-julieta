// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package termsafe_test

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// span is an inclusive range of code points.
type span struct{ lo, hi rune }

// escapedClasses are the classes of I29 (05 5.2), one row each, written
// here from the specification and not from the package, so that the
// table checks the package rather than repeating it. Bidi is the
// Bidi_Control set of Unicode (UAX #9).
var escapedClasses = []struct {
	name  string
	spans []span
}{
	{"control", []span{{0x00, 0x1F}, {0x7F, 0x9F}}},
	{"bidi", []span{{0x061C, 0x061C}, {0x200E, 0x200F}, {0x202A, 0x202E}, {0x2066, 0x2069}}},
	{"zero-width", []span{{0x200B, 0x200D}, {0x2060, 0x2060}, {0xFEFF, 0xFEFF}}},
	{"tag characters", []span{{0xE0000, 0xE007F}}},
	{"line separator U+2028", []span{{0x2028, 0x2028}}},
	{"paragraph separator U+2029", []span{{0x2029, 0x2029}}},
}

// inEscapedClass reports whether r is in one of escapedClasses.
func inEscapedClass(r rune) bool {
	for _, c := range escapedClasses {
		for _, s := range c.spans {
			if s.lo <= r && r <= s.hi {
				return true
			}
		}
	}
	return false
}

// TestEscape checks every code point of every class of I29: each one,
// between two letters, comes out as a visible escape that starts with
// a backslash and holds only printable ASCII, and the letters around
// it are kept.
//
//romeu:invariant I29 test
func TestEscape(t *testing.T) {
	for _, c := range escapedClasses {
		t.Run(c.name, func(t *testing.T) {
			for _, s := range c.spans {
				for r := s.lo; r <= s.hi; r++ {
					got := termsafe.Escape("a" + string(r) + "b")
					require.NotContains(t, got, string(r), "U+%04X", r)
					require.True(t, strings.HasPrefix(got, `a\`), "U+%04X: %q", r, got)
					require.True(t, strings.HasSuffix(got, "b"), "U+%04X: %q", r, got)
					require.True(t, isPrintableASCII(got), "U+%04X: %q", r, got)
				}
			}
		})
	}
}

// isPrintableASCII reports whether s holds only U+0020 to U+007E.
func isPrintableASCII(s string) bool {
	for i := range len(s) {
		if s[i] < 0x20 || s[i] > 0x7E {
			return false
		}
	}
	return true
}

// TestEscapeForm pins the form of each escape: the short JSON escapes
// for the five controls JSON names, \u and four lowercase hex digits
// for the other code points of the basic plane, a UTF-16 surrogate pair
// for a tag character, and \x and two hex digits for each byte that is
// not part of valid UTF-8.
func TestEscapeForm(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"nul", "\x00", `\u0000`},
		{"backspace", "\b", `\b`},
		{"tab", "\t", `\t`},
		{"line feed", "\n", `\n`},
		{"form feed", "\f", `\f`},
		{"carriage return", "\r", `\r`},
		{"escape sequence", "\x1b[31mred", `\u001b[31mred`},
		{"delete", "\x7f", `\u007f`},
		{"c1 next line", "\u0085", `\u0085`},
		{"c1 control sequence introducer", "\u009b", `\u009b`},
		{"arabic letter mark", "\u061c", `\u061c`},
		{"right-to-left override", "\u202e", `\u202e`},
		{"pop directional isolate", "\u2069", `\u2069`},
		{"zero width space", "\u200b", `\u200b`},
		{"word joiner", "\u2060", `\u2060`},
		{"byte order mark", "\ufeff", `\ufeff`},
		{"line separator", "\u2028", `\u2028`},
		{"paragraph separator", "\u2029", `\u2029`},
		{"tag latin capital a", "\U000E0041", `\udb40\udc41`},
		{"cancel tag", "\U000E007F", `\udb40\udc7f`},
		{"lone continuation byte", "\x80", `\x80`},
		{"8-bit csi byte", "\x9b", `\x9b`},
		{"invalid byte", "a\xffb", `a\xffb`},
		{"truncated sequence", "\xe2\x80", `\xe2\x80`},
		{"surrogate half encoded", "\xed\xa0\x80", `\xed\xa0\x80`},
		{"mixed", "ref\u202e\x1b\nname", `ref\u202e\u001b\nname`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, termsafe.Escape(tt.in))
		})
	}
}

// TestEscapeKeepsOtherText checks that text with no rune of I29 comes
// back unchanged, the code points next to each class included, so that
// no class is wider than the specification.
func TestEscapeKeepsOtherText(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"ascii", "refs/heads/feat/t007 ok: 42 ~!@#$%^&*()_+{}|:<>?"},
		{"backslash text", `a\u202e\x1b\n`},
		{"accents and scripts", "ol\u00e1, Julieta; \u65e5\u672c\u8a9e; \u05e2\u05d1\u05e8\u05d9\u05ea; \u0627\u0644\u0639\u0631\u0628\u064a\u0629"},
		{"emoji", "\U0001F600 \U0001F44D\U0001F3FD"},
		{"replacement character", "\ufffd"},
		{"space and tilde", " ~"},
		{"no-break space after c1", "\u00a0"},
		{"around the arabic letter mark", "\u061b\u061d"},
		{"hair space before zero width", "\u200a"},
		{"hyphen after the marks", "\u2010"},
		{"before the separators", "\u2027"},
		{"after the overrides", "\u202f"},
		{"function application after the word joiner", "\u2061"},
		{"after the isolates", "\u206a"},
		{"before the byte order mark", "\ufefe"},
		{"after the byte order mark", "\uff00"},
		{"before the tag block", "\U000DFFFF"},
		{"after the tag block", "\U000E0080"},
		{"last code point", "\U0010FFFF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.in, termsafe.Escape(tt.in))
		})
	}
}

// FuzzEscape checks, for any input, that the output is valid UTF-8,
// holds no rune of I29, comes back unchanged from a second Escape, and
// equals the input when the input is valid UTF-8 with no such rune.
// The seeds are its committed corpus; they run as a unit test in every
// CI (10 10.1, Fuzz).
func FuzzEscape(f *testing.F) {
	for _, seed := range []string{
		"",
		"plain text",
		"\x1b[2J\x1b[H",
		"\x1b]0;title\x07",
		"\u009b31m",
		"main\u202eexe.txt",
		"a\u2066b\u2069c",
		"zero\u200bwidth\ufeff",
		"\U000E0001\U000E0041\U000E007F",
		"line\u2028para\u2029",
		"bad\xff\xfe\x80",
		"\xe2\x80",
		`already \u001b escaped`,
		"tab\tnew\nline\rreturn",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		out := termsafe.Escape(in)
		require.True(t, utf8.ValidString(out), "%q", out)
		for _, r := range out {
			require.False(t, inEscapedClass(r), "U+%04X in %q", r, out)
		}
		require.Equal(t, out, termsafe.Escape(out), "a second escape")
		if utf8.ValidString(in) && !strings.ContainsFunc(in, inEscapedClass) {
			require.Equal(t, in, out)
		}
	})
}

// ExampleEscape shows a ref name with a right-to-left override and an
// escape sequence, as the host prints it.
func ExampleEscape() {
	fmt.Println(termsafe.Escape("refs/heads/fix\u202e\x1b[8m"))
	// Output: refs/heads/fix\u202e\u001b[8m
}
