// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise

import "fmt"

// MaxDepth is the deepest nesting of arrays and inline tables a mise.toml
// or a mise.lock may hold. The decoder allocates for each level, so a file
// of the size cap that is all brackets could exhaust memory; the files
// mise writes and reads nest a few levels (the audit of 2026-10-22).
const MaxDepth = 32

// MaxKeyParts is the most parts a dotted key or a table header of a
// mise.toml or a mise.lock may have. The decoder allocates for each part
// of a key once more for each part before it, so a key of 10,000 parts in
// 20 KB took 7.7 GB. The keys mise writes have three parts
// (tools.gh."platforms.linux-x64"), so 8 leaves room for what a newer mise
// adds; measured at 1 MiB of such keys, a read allocates 144 MB at 3
// parts and 316 MB at 8, where 31 parts took 1.3 GB, so the cap bounds
// the work by the size of the file (1 MiB stays, as 07 7.5 and REQ-0406
// say: a real file is far under it, and a lower cap is a decision for
// that text).
const MaxKeyParts = 8

// CheckShape refuses data whose brackets and braces nest deeper than
// MaxDepth, or that holds a key of more than MaxKeyParts parts, counting
// outside strings and comments: the basic and the literal string, each in
// its single-line and its multi-line form, and a comment to the end of
// its line. The parts of a key are counted by its dots, in a key-value
// line, in a table header and in an inline table, from the start of a
// line, a comma, an equals sign or an opening brace to the next of them,
// so a quoted segment is one part and a number such as 1.5 after an
// equals sign or a comma has one dot. It reads the bytes once and
// allocates nothing, so it can run before the decoder does. It does not
// judge whether the document is TOML, and it may count the dots of a
// value that is not valid TOML as parts of a key, which only refuses more.
func CheckShape(data []byte) error {
	depth, dots := 0, 0
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
			if c == '{' {
				dots = 0
			}
			i++
		case ']', '}':
			depth = max(depth-1, 0)
			dots = 0
			i++
		case '.':
			if dots++; dots >= MaxKeyParts {
				return fmt.Errorf("a key of more than %d key parts", MaxKeyParts)
			}
			i++
		case '\n', ',', '=':
			dots = 0
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
