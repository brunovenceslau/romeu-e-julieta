// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise

import "fmt"

// MaxDepth is the deepest nesting of arrays and inline tables a mise.toml
// or a mise.lock may hold. The decoder allocates for each level, so a file
// of the size cap that is all brackets could exhaust memory; the files
// mise writes and reads nest a few levels (the audit of 2026-10-22).
const MaxDepth = 32

// CheckDepth refuses data whose brackets and braces nest deeper than
// MaxDepth, counting outside strings and comments: the basic and the
// literal string, each in its single-line and its multi-line form, and a
// comment to the end of its line. It reads the bytes once and allocates
// nothing, so it can run before the decoder does. It does not judge
// whether the document is TOML.
func CheckDepth(data []byte) error {
	depth := 0
	for i := 0; i < len(data); {
		c := data[i]
		switch c {
		case '#':
			for i < len(data) && data[i] != '\n' {
				i++
			}
		case '"', '\'':
			i = skipString(data, i)
		case '[', '{':
			depth++
			if depth > MaxDepth {
				return fmt.Errorf("nesting deeper than %d", MaxDepth)
			}
			i++
		case ']', '}':
			depth = max(depth-1, 0)
			i++
		default:
			i++
		}
	}
	return nil
}

// skipString returns the index after the string that starts at i, a quote
// of data: a triple quote starts a multi-line string, which ends at the
// next triple quote; a basic string ("...") skips the character after a
// backslash. A string that is not closed ends at the end of the line, or
// of the data for a multi-line one.
func skipString(data []byte, i int) int {
	q := data[i]
	if i+2 < len(data) && data[i+1] == q && data[i+2] == q {
		for j := i + 3; j < len(data); j++ {
			switch {
			case q == '"' && data[j] == '\\':
				j++
			case data[j] == q && j+2 < len(data) && data[j+1] == q && data[j+2] == q:
				return j + 3
			}
		}
		return len(data)
	}
	for j := i + 1; j < len(data); j++ {
		switch {
		case q == '"' && data[j] == '\\':
			j++
		case data[j] == q:
			return j + 1
		case data[j] == '\n':
			return j
		}
	}
	return len(data)
}
