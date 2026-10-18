// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// Printer prints the verdict of each check as it ends, and remembers
// whether every one passed. Every check runs, so one run shows
// everything there is to fix.
type Printer struct {
	out io.Writer
	now func() time.Time
	// OK says whether every check reported so far passed.
	OK bool
	// Failed names the checks that failed, in the order they were
	// reported, each once.
	Failed []string
}

// DurationStep is what a duration on a verdict line is rounded to.
const DurationStep = 100 * time.Millisecond

// FormatDuration returns d rounded to DurationStep, as "1.2s".
func FormatDuration(d time.Duration) string {
	return d.Round(DurationStep).String()
}

// NewPrinter returns a Printer that writes to out and reads the time
// from time.Now.
func NewPrinter(out io.Writer) *Printer {
	return NewPrinterAt(out, time.Now)
}

// NewPrinterAt is NewPrinter with the clock now, which tests inject
// (10 10.4).
func NewPrinterAt(out io.Writer, now func() time.Time) *Printer {
	return &Printer{out: out, now: now, OK: true}
}

// Report prints one verdict of a check that took d, with what the check
// found below it.
func (c *Printer) Report(name string, passed bool, detail string, d time.Duration) {
	verdict := "ok  "
	if !passed {
		verdict, c.OK = "FAIL", false
		if !slices.Contains(c.Failed, name) {
			c.Failed = append(c.Failed, name)
		}
	}
	Say(c.out, "%s  %s (%s)\n%s", verdict, name, FormatDuration(d), detail)
}

// Last prints the last line of a run that failed, which names each
// failed check, so that no run ends with a bare "exit status 1". It
// prints nothing when every check passed. command is "fast" or "all".
func (c *Printer) Last(command string) {
	if len(c.Failed) > 0 {
		Say(c.out, "%s: failed: %s\n", command, strings.Join(c.Failed, ", "))
	}
}

// Verdict reports one verdict of a check of the commit that took d: one
// that could not judge fails with its reason, and one with a note
// prints it with "--".
func (c *Printer) Verdict(v Verdict, d time.Duration) {
	switch {
	case v.Note != "":
		Say(c.out, "--    %s: %s\n", v.Name, v.Note)
	case v.Err != nil:
		c.Report(v.Name, false, git.SafeLines(v.Err.Error())+"\n", d)
	default:
		c.Report(v.Name, len(v.Findings) == 0, Lines(v.Findings)+v.Info, d)
	}
}

// moduleLookupOff is what the go command prints when a step needs a
// module that the module cache does not hold: the steps run with
// GOPROXY=off (StepEnv).
const moduleLookupOff = "module lookup disabled by GOPROXY=off"

// Run runs each step in root and reports it. A step that failed for a
// module missing from the module cache names the command that fills
// the cache.
func (c *Printer) Run(ctx context.Context, root string, steps []Step) {
	for _, s := range steps {
		if s.Skip != "" {
			Say(c.out, "--    %s: %s\n", s.Name, s.Skip)
			continue
		}
		start := c.now()
		out, err := s.Run(ctx, root)
		detail := ""
		if err != nil {
			// out is the step's own output, which is printed as it is, for
			// readability; the unit step in particular lets the change print
			// anything. A quiet step is the exception: gofmt -l prints paths
			// of the repository and its parse errors about them, so escaping
			// its output costs no readability and is defense in depth.
			// err, the error of the run, may hold a path and is made safe
			// too.
			text := string(out)
			if s.Quiet {
				text = git.SafeLines(text)
			}
			detail = fmt.Sprintf("%s%s\n", text, git.SafeLines(err.Error()))
			if s.NoticeOn != "" && hasLinePrefix(out, s.NoticeOn) {
				detail += s.Notice + "\n"
			}
			if bytes.Contains(out, []byte(moduleLookupOff)) {
				detail += "the module cache lacks a module of go.sum, and no step fetches one (GOPROXY=off): run \"go run ./tools/ci setup\", which runs \"go mod download\", then run this again\n"
			}
		}
		c.Report(s.Name, err == nil, detail, c.now().Sub(start))
	}
}

// hasLinePrefix reports whether a line of out starts with prefix.
func hasLinePrefix(out []byte, prefix string) bool {
	for line := range bytes.Lines(out) {
		if bytes.HasPrefix(line, []byte(prefix)) {
			return true
		}
	}
	return false
}
