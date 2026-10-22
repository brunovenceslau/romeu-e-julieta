// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check

import (
	"os"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// onRunner reports whether this process runs in GitHub Actions, which
// reads a workflow command from the standard output of a step.
func onRunner() bool { return os.Getenv("GITHUB_ACTIONS") == "true" }

// Written by hand, after a search for a maintained library (T158): the
// Go modules that write workflow commands (github.com/sethvargo/
// go-githubactions, Apache-2.0, v1.4.0) bring a client for the whole
// runner (outputs, summaries, the OIDC token) for the 10 lines below,
// and a new requirement is ask-first. Reopened by a second consumer of a
// workflow command in this repository.
var (
	dataEscaper     = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	propertyEscaper = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
)

// Annotation returns the workflow command that shows msg on the run as
// an annotation of the level ("warning", "notice" or "error") with the
// title title: "::<level> title=<title>::<msg>", with the escaping of
// actions/toolkit, command.ts: the message escapes "%", CR and LF, the
// title ":" and "," as well. The message goes through git.SafeLines
// first, which breaks the legacy "##[" marker the runner finds anywhere
// in a line (actions/runner, ActionCommand.cs), so the result is one
// line that starts a command of its own and holds no other.
func Annotation(level, title, msg string) string {
	return "::" + level + " title=" + propertyEscaper.Replace(title) + "::" + dataEscaper.Replace(git.SafeLines(msg))
}

// warn prints msg, a note of the step name: an annotation on a runner,
// and a line that starts with "warn" elsewhere, through git.SafeLines.
func (c *Printer) warn(name, msg string) {
	if onRunner() {
		Say(c.out, "%s\n", Annotation("warning", name, msg))
		return
	}
	Say(c.out, "warn  %s: %s\n", name, git.SafeLines(msg))
}
