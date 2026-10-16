// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// Printer prints the verdict of each check as it ends, and remembers
// whether every one passed. Every check runs, so one run shows
// everything there is to fix.
type Printer struct {
	out io.Writer
	// OK says whether every check reported so far passed.
	OK bool
}

// NewPrinter returns a Printer that writes to out.
func NewPrinter(out io.Writer) *Printer {
	return &Printer{out: out, OK: true}
}

// Report prints one verdict, with what the check found below it.
func (c *Printer) Report(name string, passed bool, detail string) {
	verdict := "ok  "
	if !passed {
		verdict, c.OK = "FAIL", false
	}
	Say(c.out, "%s  %s\n%s", verdict, name, detail)
}

// Verdict reports one verdict of a check of the commit: one that could not
// judge fails with its reason, and one with a note prints it with
// "--".
func (c *Printer) Verdict(v Verdict) {
	switch {
	case v.Note != "":
		Say(c.out, "--    %s: %s\n", v.Name, v.Note)
	case v.Err != nil:
		c.Report(v.Name, false, git.SafeLines(v.Err.Error())+"\n")
	default:
		c.Report(v.Name, len(v.Findings) == 0, Lines(v.Findings)+v.Info)
	}
}

// moduleLookupOff is what the go command prints when a step needs a
// module that the module cache does not hold: the steps run with
// GOPROXY=off (stepEnv).
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
			if bytes.Contains(out, []byte(moduleLookupOff)) {
				detail += "the module cache lacks a module of go.sum, and no step fetches one (GOPROXY=off): run \"go run ./tools/ci setup\", which runs \"go mod download\", then run this again\n"
			}
		}
		c.Report(s.Name, err == nil, detail)
	}
}
