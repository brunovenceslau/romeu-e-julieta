// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package approval

import (
	"fmt"
	"io"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// Finding is one thing pr reports: where, what, and, when the finding is
// about a line the pull request's author wrote, that line, which Print
// writes on a line of its own behind quotePrefix.
type Finding struct {
	Where string
	Msg   string
	Quote string
}

// quotePrefix starts every printed line of the author's text (REQ-1153):
// no line pr prints starts with the author's bytes, so none reads as a
// workflow command, which the Actions runner looks for at the start of a
// line, after blanks.
const quotePrefix = "> "

// rule is the name each finding line carries, as the findings of
// tools/ci do (check.Finding).
const rule = "ask-first"

// String prints a finding as tools/ci prints its own, "<where>: pr:
// <msg>", passed through git.SafeLines, and then the quoted line, if any.
func (f Finding) String() string {
	s := git.SafeLines(git.Printable(f.Where) + ": " + rule + ": " + f.Msg)
	if f.Quote != "" {
		s += "\n" + Quote(f.Quote)
	}
	return s
}

// Quote returns one line of the author's text as pr prints it (REQ-1153):
// quotePrefix, then the text with every line break and every character
// that does not print written as an escape, so the text stays on this
// one line, and "##[", the older form of a workflow command, escaped
// (git.SafeLines).
func Quote(text string) string {
	text = strings.NewReplacer("\r", `\r`, "\n", `\n`).Replace(text)
	return git.SafeLines(quotePrefix + text)
}

// Print writes the findings and then the summary line, "pr: ok" or the
// number of findings, as a subcommand of tools/ci does, and reports
// whether there were none. A write that fails has nowhere left to be
// reported, so its error is dropped.
func Print(w io.Writer, findings []Finding) bool {
	var b strings.Builder
	for _, f := range findings {
		b.WriteString(f.String() + "\n")
	}
	if len(findings) == 0 {
		b.WriteString(rule + ": ok\n")
	} else {
		fmt.Fprintf(&b, "%s: %d finding(s)\n", rule, len(findings))
	}
	_, _ = io.WriteString(w, b.String())
	return len(findings) == 0
}
