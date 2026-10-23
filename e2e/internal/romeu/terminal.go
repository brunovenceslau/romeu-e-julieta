// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build linux || darwin

package romeu

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// lockedBuffer is a buffer the harness reads while romeu writes it.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

// Write appends p.
func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

// String returns what was written so far.
func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// TerminalDeadline is how long RunTerminal and RunStdinTerminal let
// romeu run: a romeu still running then is ended (its process group is
// killed), and the test fails with the screen so far. It is far above
// what any prompt of the e2e takes, and far below the test timeout.
const TerminalDeadline = 60 * time.Second

// RunTerminal is Run with the standard input and the standard error of
// romeu on the terminal of a /dev/ptmx pair (10 10.1, REQ-0790: a test
// helper over x/sys/unix, no pty dependency), as an operator runs it,
// and its standard output captured apart, so a test sees which stream a
// prompt went to. Result.Stderr is the screen: what romeu wrote to the
// terminal, with the echo of what was typed, line ends as "\n". It types
// answer only once the screen shows prompt, the question romeu writes
// after it drops what was typed before (internal/gate), so the answer is
// never dropped; an empty prompt types nothing. A romeu that has not
// ended by TerminalDeadline fails the test.
func (e *Env) RunTerminal(tb testing.TB, prompt, answer string, args []string, extra ...string) Result {
	tb.Helper()
	return e.RunTerminalWithin(tb, TerminalDeadline, prompt, answer, args, extra...)
}

// RunTerminalWithin is RunTerminal with the deadline given.
func (e *Env) RunTerminalWithin(tb testing.TB, deadline time.Duration, prompt, answer string, args []string, extra ...string) Result {
	tb.Helper()
	return e.runOnTerminal(tb, deadline, true, prompt, answer, args, extra...)
}

// RunStdinTerminal is Run with only the standard input of romeu on a
// terminal, and its standard output and error captured apart: the shell
// line romeu ... 2>log. It types nothing.
func (e *Env) RunStdinTerminal(tb testing.TB, args []string, extra ...string) Result {
	tb.Helper()
	return e.runOnTerminal(tb, TerminalDeadline, false, "", "", args, extra...)
}

// runOnTerminal runs romeu with stdin on a terminal, and stderr too when
// stderrOnTerminal is set, for at most deadline.
func (e *Env) runOnTerminal(tb testing.TB, deadline time.Duration, stderrOnTerminal bool, prompt, answer string, args []string, extra ...string) Result {
	tb.Helper()
	master, terminal := openPTY(tb)
	// The screen is everything the terminal shows; reading it also keeps
	// the terminal's output queue from filling. The read ends when the
	// terminal is closed and the queue is drained.
	var screen lockedBuffer
	drained := make(chan struct{})
	go func() {
		_, _ = io.Copy(&screen, master)
		close(drained)
	}()

	var stdout, stderr lockedBuffer
	var errOut io.Writer = &stderr
	shown := stderr.String
	if stderrOnTerminal {
		errOut = terminal
		shown = screen.String
	}
	ctx, cancel := context.WithTimeout(tb.Context(), deadline)
	defer cancel()
	typed := make(chan error, 1)
	go func() {
		if prompt == "" {
			typed <- nil
			return
		}
		for !strings.Contains(shown(), prompt) {
			select {
			case <-ctx.Done():
				typed <- nil // the run ends, and reports why
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
		_, err := io.WriteString(master, answer)
		typed <- err
	}()
	err := procgroup.Run(ctx, procgroup.Command{
		Class:  procgroup.ToolStep,
		Argv:   append([]string{e.Romeu}, args...),
		Dir:    e.Home,
		Env:    e.Environ(extra...),
		Stdin:  terminal,
		Stdout: &stdout, Stderr: errOut,
	})
	expired := ctx.Err() != nil
	cancel()
	terr := <-typed
	_ = terminal.Close()
	// After the terminal is closed the read ends once the queue is read.
	// A master that never reports the end is closed, and the screen is
	// what was read by then; the run never waits on the read for long.
	select {
	case <-drained:
	case <-time.After(5 * time.Second):
		_ = master.Close()
	}
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if stderrOnTerminal {
		res.Stderr = strings.ReplaceAll(screen.String(), "\r\n", "\n")
	}
	switch {
	case expired:
		tb.Fatalf("romeu %v did not end in %s and was killed; the screen shows %q, stdout %q", args, deadline, res.Stderr, res.Stdout)
	case terr != nil:
		tb.Fatalf("romeu %v: type the answer: %v; the screen shows %q", args, terr, res.Stderr)
	}
	if err == nil {
		return res
	}
	if exit, ok := errors.AsType[*procgroup.ExitError](err); ok {
		res.Code = exit.Code
		return res
	}
	tb.Fatalf("romeu %v: %v", args, err)
	return res
}
