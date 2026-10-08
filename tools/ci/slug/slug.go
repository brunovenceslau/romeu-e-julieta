// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package slug holds the one function that turns an ADR title into the
// words of its filename (rule 6 of ADR 0001). The author of a record
// (tools/new) and the check of the layout (tools/ci sequences) both call
// it, so they cannot disagree about a filename.
package slug

import "strings"

// Slug lowercases the ASCII letters of title, turns each run of other
// characters into one hyphen, and trims the hyphens from both ends. Only
// ASCII letters and digits survive: a letter outside ASCII is a
// separator, so the result does not depend on a locale or on a Unicode
// table. A title with no ASCII letter or digit gives the empty string.
func Slug(title string) string {
	var b strings.Builder
	hyphen := false
	for i := 0; i < len(title); i++ {
		c := title[i]
		switch {
		case c >= 'A' && c <= 'Z':
			c += 'a' - 'A'
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		default:
			hyphen = b.Len() > 0
			continue
		}
		if hyphen {
			b.WriteByte('-')
			hyphen = false
		}
		b.WriteByte(c)
	}
	return b.String()
}
