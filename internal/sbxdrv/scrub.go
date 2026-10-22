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

// Replace returns text with the four values replaced. Paths are replaced
// first, the longest first, since the root usually lies in the home;
// the user and host names are replaced where they stand as a whole word,
// so that a short name does not cut up other words.
func (s Scrubber) Replace(text string) string {
	paths := []struct{ value, placeholder string }{{s.Root, PlaceholderRoot}, {s.Home, PlaceholderHome}}
	slices.SortStableFunc(paths, func(a, b struct{ value, placeholder string }) int { return len(b.value) - len(a.value) })
	for _, p := range paths {
		if p.value != "" {
			text = strings.ReplaceAll(text, p.value, p.placeholder)
		}
	}
	for _, w := range []struct{ value, placeholder string }{{s.Host, PlaceholderHost}, {s.User, PlaceholderUser}} {
		if w.value != "" {
			text = replaceWord(text, w.value, w.placeholder)
		}
	}
	return text
}

// replaceWord replaces each occurrence of word in text that no letter,
// digit, underscore or hyphen touches.
func replaceWord(text, word, repl string) string {
	var b strings.Builder
	for {
		i := strings.Index(text, word)
		if i < 0 {
			break
		}
		end := i + len(word)
		if (i == 0 || !wordByte(text[i-1])) && (end == len(text) || !wordByte(text[end])) {
			b.WriteString(text[:i])
			b.WriteString(repl)
		} else {
			b.WriteString(text[:end])
		}
		text = text[end:]
	}
	b.WriteString(text)
	return b.String()
}

// wordByte reports whether c is part of a word of a user or host name.
func wordByte(c byte) bool {
	return c == '_' || c == '-' || c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= 0x80
}
