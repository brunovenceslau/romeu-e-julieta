// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package sbxdrv

import (
	"slices"
	"strings"
)

// The placeholders that stand for the host's own names in the details
// of a failed call (04 4.1, Error ids, item 7) and in a recording
// (10 10.3, Redaction). The recorder (T018) writes the same four.
const (
	PlaceholderHome = "<home>"
	PlaceholderUser = "<user>"
	PlaceholderHost = "<host>"
	PlaceholderRoot = "<root>"
)

// The words that stand for a value the details of a failed call never
// show: the host command of a command secret, and the value of --env.
// They are the tokens of placeholders.go.
const (
	PlaceholderCommand = "<command>"
	PlaceholderEnv     = "<env>"
)

// Scrubber replaces the home path, the user name, the host name and the
// root path of the host tree, as the recorder of 10 10.3 does, so that
// text from sbx can be pasted into an issue. A value left empty replaces
// nothing.
type Scrubber struct {
	Home string // the home directory of the user
	User string // the user name
	Host string // the host name
	Root string // the root of the host tree, $ROMEU_ROOT
}

// Replace returns text with the four values replaced. The text is read
// once, left to right, and what a replacement writes is never read
// again, so a user named home or root does not change <home> or <root>.
// At each place the paths go first, the longest first, since the root
// usually lies in the home; the host and user names are replaced where
// they stand as a whole word, so that a short name does not cut up other
// words. A value left empty replaces nothing, and so does "/", which is
// the start of every path and not a name of the host's.
func (s Scrubber) Replace(text string) string {
	type rule struct {
		value, placeholder string
		word               bool
	}
	var rules []rule
	for _, p := range []rule{{s.Root, PlaceholderRoot, false}, {s.Home, PlaceholderHome, false}} {
		if p.value != "" && p.value != "/" {
			rules = append(rules, p)
		}
	}
	slices.SortStableFunc(rules, func(a, b rule) int { return len(b.value) - len(a.value) })
	for _, w := range []rule{{s.Host, PlaceholderHost, true}, {s.User, PlaceholderUser, true}} {
		if w.value != "" {
			rules = append(rules, w)
		}
	}
	var b strings.Builder
	for i := 0; i < len(text); {
		matched := false
		for _, r := range rules {
			if !strings.HasPrefix(text[i:], r.value) {
				continue
			}
			end := i + len(r.value)
			if r.word && ((i > 0 && wordByte(text[i-1])) || (end < len(text) && wordByte(text[end]))) {
				continue
			}
			b.WriteString(r.placeholder)
			i, matched = end, true
			break
		}
		if !matched {
			b.WriteByte(text[i])
			i++
		}
	}
	return b.String()
}

// wordByte reports whether c is part of a word of a user or host name.
func wordByte(c byte) bool {
	return c == '_' || c == '-' || c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= 0x80
}
