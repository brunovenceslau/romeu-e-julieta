// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package changelog embeds CHANGELOG.md, the generated changelog of 12
// 12.6, for romeu upgrade (04 4.2, REQ-1348). It is the one package at
// the root of the repository (02 2.1, REQ-0129): go:embed does not
// cross a parent directory, and the file stays at the root, where
// people and git-cliff read it.
package changelog

import (
	_ "embed"
	"regexp"
	"strings"
)

//go:embed CHANGELOG.md
var file string

// File returns the embedded CHANGELOG.md.
func File() string { return file }

// heading is a level-2 heading at the start of a line, which only a
// section begins (REQ-1350): the text of an entry is indented and the
// Upgrading part uses level 3. tools/release/internal/changelog reads
// the file by the same rule; a product package cannot import it.
var heading = regexp.MustCompile(`(?m)^## (\S+)[ \t]*$`)

// Since returns the sections of text that romeu upgrade prints
// (REQ-1348): every section after the one of running up to the one of
// own, newest first, as the file holds them, each from its level-2
// heading to the next one or the end of the file (REQ-1350), its
// trailing blank lines removed and the sections joined by one blank
// line. When running is not a tag of the file, a dev build
// (v0.0.0-dev+<commit>, REQ-0065) or a version older than the file, or
// is not older than own, it returns the section of own alone. It
// returns "" when own is not a tag of the file: a dev build has no
// section. The text is returned as the file holds it; the caller
// escapes it (I29).
func Since(text, running, own string) string {
	type section struct{ tag, body string }
	var sections []section
	all := heading.FindAllStringSubmatchIndex(text, -1)
	for i, m := range all {
		end := len(text)
		if i+1 < len(all) {
			end = all[i+1][0]
		}
		sections = append(sections, section{tag: text[m[2]:m[3]], body: strings.TrimRight(text[m[0]:end], "\n")})
	}
	from, to := -1, -1
	for i, s := range sections {
		switch s.tag {
		case own:
			from = i
		case running:
			to = i
		}
	}
	if from < 0 {
		return ""
	}
	if to <= from {
		return sections[from].body
	}
	bodies := make([]string, 0, to-from)
	for _, s := range sections[from:to] {
		bodies = append(bodies, s.body)
	}
	return strings.Join(bodies, "\n\n")
}
