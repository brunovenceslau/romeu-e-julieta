// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check

import (
	"bytes"
	"fmt"
	"io"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// Finding is one broken rule. It names a place and a rule and never
// repeats the text that matched: CI logs are public, and the output is
// pasted into pull requests.
type Finding struct {
	Where string
	Rule  string
	Msg   string
}

// String prints a finding with its path quoted by git.Printable when it
// does not print, and the whole line passed through git.SafeLines, so
// repository content cannot forge a line of the log or a workflow command.
func (f Finding) String() string {
	return git.SafeLines(git.Printable(f.Where) + ": " + f.Rule + ": " + f.Msg)
}

// Verdict is what one check of the commit judged found: its findings,
// or why it could not judge (Err), or, with a note, why it did not
// check and so neither passed nor failed; Info is a line printed under
// the verdict either way, for what passed and is still worth a read.
type Verdict struct {
	Name     string
	Findings []Finding
	Err      error
	Note     string
	Info     string
}

// Lines returns the findings, one per line.
func Lines(findings []Finding) string {
	var b bytes.Buffer
	for _, f := range findings {
		fmt.Fprintln(&b, f)
	}
	return b.String()
}

// Say writes one piece of output. A write to the terminal or to a pipe
// that fails has nowhere left to be reported, so the error is dropped
// here, in one place.
func Say(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

// Summary is the last line a subcommand prints: ok, or the number of
// findings.
func Summary(check string, findings int) string {
	if findings == 0 {
		return check + ": ok"
	}
	return fmt.Sprintf("%s: %d finding(s)", check, findings)
}
