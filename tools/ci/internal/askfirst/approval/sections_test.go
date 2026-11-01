// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package approval

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// fullBody is a body with the four sections and text under each, in
// the form the pull request template gives.
const fullBody = "## Why\n\nA reason.\n\n## What changed\n\n- a file\n\n## Evidence\n\n`go test`: ok\n\n## Middleware\n\nnone\n\n## Lessons\n\nnone\n"

// The tails of the findings of REQ-1131.
const (
	noHeading = "(12 12.4, REQ-1131): no heading"
	noText    = "(12 12.4, REQ-1131): no text under the heading"
)

// finding is the finding of a section with the given tail.
func finding(name, tail string) string {
	return "the body needs the section \"## " + name + "\", a level-2 heading with text under it " + tail
}

// with replaces one part of fullBody.
func with(old, repl string) string {
	return strings.Replace(fullBody, old, repl, 1)
}

// TestSections lists what the section check of REQ-1131 finds, one
// fixture per case: each finding names the section and 12 12.4.
func TestSections(t *testing.T) {
	const alertInWhatChanged = "- a file\n\n> [!IMPORTANT]\n> - [x] Approval: decisions - x\n> - [ ] Approval: checks - y\n"
	cases := []struct {
		name string
		body string
		want []string // the findings' messages, in order
	}{
		{"the four sections with text", fullBody, nil},
		{"Lessons with a lesson", with("## Lessons\n\nnone\n", "## Lessons\n\nRead the lock before the write.\n"), nil},
		{"Lessons with none alone", "## Why\n\nx\n\n## What changed\n\nx\n\n## Evidence\n\nx\n\n## Lessons\n\nnone\n", nil},
		{"Middleware absent", with("## Middleware\n\nnone\n\n", ""), nil},
		{"the first line a heading", "## Why\nx\n## What changed\nx\n## Evidence\nx\n## Lessons\nnone\n", nil},
		{"the alert inside What changed, as the template puts it", with("- a file\n", alertInWhatChanged), nil},
		{"the alert after Lessons", fullBody + "\n> [!IMPORTANT]\n> - [x] Approval: decisions - x\n", nil},
		{"the alert before Why", "> [!IMPORTANT]\n> - [x] Approval: decisions - x\n\n" + fullBody, nil},
		{"text after the alert ends counts", with("- a file\n", "> [!IMPORTANT]\n> - [x] Approval: decisions - x\n\n- a file\n"), nil},
		{"two alerts", with("- a file\n", alertInWhatChanged) + "\n> [!IMPORTANT]\n> - [x] Approval: kits - z\n", nil},
		{"trailing blanks after a heading", with("## Why\n", "## Why  \n"), nil},
		{"CR LF line ends", strings.ReplaceAll(fullBody, "\n", "\r\n"), nil},
		{"CR LF line ends with an HTML comment", strings.ReplaceAll(with("A reason.", "<!-- the template's note -->\nA reason."), "\n", "\r\n"), nil},
		{"a quote block inside Why", with("A reason.", "> a quote"), nil},
		{"text inside an HTML comment is no text", with("A reason.", "<!-- A reason. -->"), []string{finding("Why", noText)}},
		{"a heading inside an HTML comment is no heading", with("## Why", "<!-- ## Why -->"), []string{finding("Why", noHeading)}},
		{"the alert with trailing blanks is text, not an alert", with("- a file\n", "> [!IMPORTANT] \n> - [x] Approval: decisions - x\n"), nil},
		{"Why in prose", with("## Why\n\nA reason.", "Why: a reason."), []string{finding("Why", noHeading)}},
		{"a heading of another level", with("## Why", "### Why"), []string{finding("Why", noHeading)}},
		{"an indented heading", with("## Why", "  ## Why"), []string{finding("Why", noHeading)}},
		{"other text after the name", with("## Why", "## Why this"), []string{finding("Why", noHeading)}},
		{"a bold line in place of the heading", with("## Why", "**Why**"), []string{finding("Why", noHeading)}},
		{"the heading twice", with("## Evidence\n", "## Evidence\n\nx\n\n## Evidence\n"), []string{finding("Evidence", "(12 12.4, REQ-1131): 2 headings")}},
		{"an empty section", with("- a file\n", ""), []string{finding("What changed", noText)}},
		{"a section of blanks", with("- a file\n", "   \n\t\n"), []string{finding("What changed", noText)}},
		{"a section whose only text is the alert", with("- a file\n", "> [!IMPORTANT]\n> - [x] Approval: decisions - x\n"), []string{finding("What changed", noText)}},
		{"Lessons empty", with("## Lessons\n\nnone\n", "## Lessons\n"), []string{finding("Lessons", noText)}},
		{"an empty body", "", []string{finding("Why", noHeading), finding("What changed", noHeading), finding("Evidence", noHeading), finding("Lessons", noHeading)}},
		{"the headings alone", "## Why\n## What changed\n## Evidence\n## Lessons\n", []string{finding("Why", noText), finding("What changed", noText), finding("Evidence", noText), finding("Lessons", noText)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			findings := sectionFindings(c.body)
			assert.Equal(t, c.want, messages(findings))
			for _, f := range findings {
				assert.Equal(t, bodyWhere, f.Where)
				assert.Empty(t, f.Quote, "a section finding quotes no line of the author")
			}
		})
	}
}
