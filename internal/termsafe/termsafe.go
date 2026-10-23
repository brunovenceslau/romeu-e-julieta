// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package termsafe makes text safe to print on the host's terminal
// (I29, 05 5.2): Escape replaces each code point of the classes I29
// names with a visible escape, so text an agent wrote cannot move the
// cursor, recolour or hide what follows, reorder a line, or hide a
// character. The classes are those of I29 and no wider:
//
//   - control characters, U+0000 to U+001F and U+007F to U+009F;
//   - the bidi controls of Unicode (Bidi_Control, UAX #9): U+061C,
//     U+200E, U+200F, U+202A to U+202E and U+2066 to U+2069;
//   - the zero-width characters U+200B to U+200D, U+2060 and U+FEFF;
//   - the tag characters, U+E0000 to U+E007F;
//   - U+2028 and U+2029, the line and paragraph separators;
//   - the spaces that look like U+0020, U+00A0, U+2000 to U+200A,
//     U+205F and U+3000, and U+2800, the braille pattern blank, which
//     looks like nothing: two lines that differ only in them would
//     print alike;
//   - the Unicode property Default_Ignorable_Code_Point, which adds
//     among others U+00AD, U+034F, U+2061 to U+2064, U+206A to U+206F,
//     the variation selectors, the Hangul fillers and the unassigned
//     code points reserved for the property. The property is derived
//     as DerivedCoreProperties.txt defines it (see isDefaultIgnorable),
//     in the Unicode version of the Go release pinned in mise.toml
//     (unicode.Version). The annotation characters U+FFF9 to U+FFFB and
//     the Egyptian hieroglyph format characters U+13430 to U+13440 are
//     excluded from the property, and the text they annotate stays
//     displayed, so they are kept.
//
// A byte that is not part of valid UTF-8 is escaped too, as \x and two
// hex digits: a terminal that reads 8-bit controls takes the byte 0x9B
// for CSI.
//
// A code point is written as its JSON escape: \b, \t, \n, \f and \r
// for the five controls JSON names, \u and four lowercase hex digits
// for the others, and a UTF-16 surrogate pair of \u escapes for a tag
// character. Inside a JSON string these are JSON's own escapes, so the
// JSON that encoding/json writes, compact or indented with spaces,
// stays valid JSON that decodes to the same value, and the --json
// output of 04 4.1 can pass through the same writer as human text. A
// tab or a carriage return between JSON tokens is escaped too, which
// breaks the JSON, so JSON output is never indented with tabs.
//
// The backslash itself is kept, so the escape of a string is not
// unique to it: the text \u202e and the code point U+202E print alike.
// What I29 asks is that nothing an agent wrote is invisible or acts on
// the terminal, and every output of Escape is that; in exchange, the
// output contains no code point Escape changes, so escaping twice
// equals escaping once and a layer that escapes again does no harm.
package termsafe

import (
	"fmt"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// escapedRanges holds the code points of I29, as inclusive ranges in
// ascending order.
var escapedRanges = []struct{ lo, hi rune }{
	{0x0000, 0x001F},   // C0 controls
	{0x007F, 0x009F},   // delete and the C1 controls
	{0x00A0, 0x00A0},   // no-break space (look-alike space)
	{0x061C, 0x061C},   // arabic letter mark (bidi)
	{0x2000, 0x200A},   // en quad to hair space (look-alike spaces)
	{0x200B, 0x200D},   // zero width space, non-joiner and joiner
	{0x200E, 0x200F},   // left-to-right and right-to-left marks (bidi)
	{0x2028, 0x2029},   // line and paragraph separators
	{0x202A, 0x202E},   // embeddings, pop and overrides (bidi)
	{0x205F, 0x205F},   // medium mathematical space (look-alike space)
	{0x2060, 0x2060},   // word joiner (zero width)
	{0x2066, 0x2069},   // isolates and their pop (bidi)
	{0x2800, 0x2800},   // braille pattern blank (looks like nothing)
	{0x3000, 0x3000},   // ideographic space (look-alike space)
	{0xFEFF, 0xFEFF},   // zero width no-break space, the byte order mark
	{0xE0000, 0xE007F}, // tag characters
}

// shortEscapes are the controls JSON writes with one letter.
var shortEscapes = map[rune]string{'\b': `\b`, '\t': `\t`, '\n': `\n`, '\f': `\f`, '\r': `\r`}

// Escape returns s with each code point of I29, and each byte that is
// not part of valid UTF-8, replaced by its visible escape; every other
// code point is kept as it is. The output is valid UTF-8, and Escape
// of the output is the output.
//
//romeu:invariant I29 guard
func Escape(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1: // not UTF-8: show the byte
			b = fmt.Appendf(b, `\x%02x`, s[i])
		case isEscaped(r):
			b = appendEscape(b, r)
		default:
			b = append(b, s[i:i+size]...)
		}
		i += size
	}
	return string(b)
}

// isDefaultIgnorable reports whether r has the Unicode property
// Default_Ignorable_Code_Point. Go has no table for it, so it is
// derived as DerivedCoreProperties.txt defines it:
//
//	Other_Default_Ignorable_Code_Point + Cf + Variation_Selector
//	  - White_Space
//	  - U+FFF9..U+FFFB (annotation characters)
//	  - U+13430..U+13440 (Egyptian hieroglyph format characters)
//	  - Prepended_Concatenation_Mark (exceptional Cf that stay visible)
func isDefaultIgnorable(r rune) bool {
	if r < 0xAD { // fast path: the first member is U+00AD
		return false
	}
	if !unicode.In(r, unicode.Other_Default_Ignorable_Code_Point, unicode.Cf, unicode.Variation_Selector) {
		return false
	}
	switch {
	case unicode.Is(unicode.White_Space, r):
		return false
	case r >= 0xFFF9 && r <= 0xFFFB:
		return false
	case r >= 0x13430 && r <= 0x13440:
		return false
	}
	return !unicode.Is(unicode.Prepended_Concatenation_Mark, r)
}

// isEscaped reports whether r is a code point of I29.
func isEscaped(r rune) bool {
	if isDefaultIgnorable(r) {
		return true
	}
	for _, e := range escapedRanges {
		if r < e.lo {
			return false
		}
		if r <= e.hi {
			return true
		}
	}
	return false
}

// appendEscape appends the JSON escape of r to b.
func appendEscape(b []byte, r rune) []byte {
	if short, ok := shortEscapes[r]; ok {
		return append(b, short...)
	}
	if r > 0xFFFF {
		hi, lo := utf16.EncodeRune(r)
		return fmt.Appendf(b, `\u%04x\u%04x`, hi, lo)
	}
	return fmt.Appendf(b, `\u%04x`, r)
}
