// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// The most of a call's output the runner keeps: far above any output
// romeu reads, so a runaway call cannot fill the memory of the host.
// What a parser accepts is below this (maxListOutput).
const (
	maxStdout = 16 << 20
	maxStderr = 64 << 10
)

// Terminal is where the streams of an Interactive call go: the
// terminal's, raw (04 4.1, Output). A nil stream is empty for stdin and
// discarded for the others.
type Terminal struct {
	In       io.Reader
	Out, Err io.Writer
}

// Runner starts sbx calls: by the absolute path from host settings,
// in the scrubbed environment of 04 4.1, under the timeout class of
// the call, through internal/procgroup. It is the only code that runs
// sbx (12 12.2).
type Runner struct {
	// Sbx is the absolute path of sbx, tools.sbx of host settings.
	Sbx string
	// Home is the HOME every call gets.
	Home string
	// Term is the TERM an Interactive call gets; empty sets none.
	Term string
	// Host is the host settings the environment contributions read; it
	// may be nil.
	Host *spec.HostSettings
	// Scrub replaces the host's own names in the stderr of a failure.
	Scrub Scrubber
	// Versions are the versions the details of a failure carry.
	Versions Versions
	// Terminal is the terminal of the Interactive calls.
	Terminal Terminal
}

// Opts are what a call knows about the command that makes it.
type Opts struct {
	// Step is the numbered step of 04 4.2 the call runs in, such as
	// "run step 3"; it enters the details of a failure.
	Step string
	// Project is the project the call is for, which the environment
	// contributions read; nil for a call of no project.
	Project *spec.Project
}

// Result is what a successful call gave.
type Result struct {
	// Stdout and Stderr are the output of a Captured call.
	Stdout, Stderr []byte
	// Truncated says that the stdout passed maxStdout and was cut.
	Truncated bool
}

// Run starts the call and waits for it. A call that succeeds returns its
// output. One that sbx fails, that cannot be started or that passes its
// timeout is a *CallError, which wraps the *procgroup.TimeoutError
// (subprocess-timeout, RJ-323) of a timeout; a call the context ended
// returns the cancellation, not a CallError, since the signal rule of
// 04 4.1 owns it. A call that breaks a guard (I4, I25) or whose
// environment is wrong, or that no builder of this package made, is the
// error internal (RJ-348), and nothing is started.
func (r *Runner) Run(ctx context.Context, c Call, o Opts) (Result, error) {
	if !filepath.IsAbs(r.Sbx) || filepath.Clean(r.Sbx) != r.Sbx {
		return Result{}, cli.Internal.New("sbxdrv: sbx is run by a clean absolute path from host settings, and the runner has " + fmt.Sprintf("%q", r.Sbx))
	}
	if !c.validated {
		return Result{}, cli.Internal.New("sbxdrv: the call " + c.Name + " was not built by a builder of this package")
	}
	if c.Mode != Captured && c.Mode != Interactive {
		return Result{}, cli.Internal.New("sbxdrv: the call " + c.Name + " has no mode")
	}
	if err := Guard(c); err != nil {
		return Result{}, err
	}
	env, err := environment(r.Home, r.Term, EnvInput{Call: c, Host: r.Host, Project: o.Project}, contributions)
	if err != nil {
		return Result{}, err
	}
	cmd := procgroup.Command{Class: c.Class, Argv: append([]string{r.Sbx}, c.Argv...), Env: env}
	var stdout, stderr capped
	stdout.max, stderr.max = maxStdout, maxStderr
	if c.Mode == Interactive {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = r.Terminal.In, r.Terminal.Out, r.Terminal.Err
	} else {
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
	}
	if err := procgroup.Run(ctx, cmd); err != nil {
		if ctx.Err() != nil {
			return Result{}, fmt.Errorf("sbx %s: %w", c.Name, err)
		}
		return Result{}, newCallError(c, err, stderr.buf.Bytes(), o.Step, r.Versions, r.Scrub)
	}
	return Result{Stdout: stdout.buf.Bytes(), Stderr: stderr.buf.Bytes(), Truncated: stdout.full}, nil
}

// capped is a buffer that keeps the first max bytes and takes the rest
// without error, so that a command that writes more is not blocked or
// killed by the writer.
type capped struct {
	buf  bytes.Buffer
	max  int
	full bool
}

// Write keeps what fits.
func (c *capped) Write(p []byte) (int, error) {
	n := len(p)
	if room := c.max - c.buf.Len(); room < n {
		c.full = true
		p = p[:max(room, 0)]
	}
	c.buf.Write(p)
	return n, nil
}
