// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package approval

import (
	"fmt"
	"strings"
)

// The sections a pull request body holds (12 12.4, REQ-1131), each under
// a level-2 heading of exactly that name: the changelog reads the Why
// section by its heading (12 12.6), and the other three are what the
// template asks. Lessons holds a lesson or the word none.
var sections = []string{"Why", "What changed", "Evidence", "Lessons"}

// headingPrefix starts a level-2 heading.
const headingPrefix = "## "

// sectionFindings applies REQ-1131 to the body and returns one finding
// per break, in the order of sections: a heading missing, written
// twice, or a section with no text under it. The body is read as the
// approval lines are (bodyLines: HTML comments removed, CR LF accepted),
// and the lines of each approval alert, which readAlert reads by the
// same rule (the line alertLine and the lines after it that start with
// ">"), belong to no section, wherever the alert sits; the template
// puts it inside What changed. A heading of another level, indented,
// or with other text after the name, is not the section's heading; one
// with trailing blanks is. A line inside a fenced code block is read
// as any other, as the rules of 12 12.4 name no fence.
func sectionFindings(body string) []Finding {
	count := map[string]int{}
	text := map[string]bool{}
	current, inAlert := "", false
	for _, l := range bodyLines(body) {
		if l == alertLine {
			inAlert = true
			continue
		}
		if inAlert = inAlert && strings.HasPrefix(l, ">"); inAlert {
			continue
		}
		if name, ok := strings.CutPrefix(l, headingPrefix); ok {
			current = strings.TrimRight(name, " \t")
			count[current]++
			continue
		}
		if current != "" && strings.TrimSpace(l) != "" {
			text[current] = true
		}
	}
	var out []Finding
	for _, name := range sections {
		msg := ""
		switch {
		case count[name] == 0:
			msg = "no heading"
		case count[name] > 1:
			msg = fmt.Sprintf("%d headings", count[name])
		case !text[name]:
			msg = "no text under the heading"
		}
		if msg != "" {
			out = append(out, Finding{Where: bodyWhere, Msg: fmt.Sprintf("the body needs the section %q, a level-2 heading with text under it (12 12.4, REQ-1131): %s", headingPrefix+name, msg)})
		}
	}
	return out
}
