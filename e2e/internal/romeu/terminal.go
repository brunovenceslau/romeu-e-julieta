// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build linux || darwin

package romeu

import (
	"bytes"
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

// promptWait is how long RunTerminal waits for the prompt, and for the
// screen to settle once romeu ended.
const promptWait = 30 * time.Second

// RunTerminal is Run with the standard input and the standard error of
// romeu on the terminal of a /dev/ptmx pair (10 10.1, REQ-0790: a test
// helper over x/sys/unix, no pty dependency), as an operator runs it,
// and its standard output captured apart, so a test sees which stream a
// prompt went to. Result.Stderr is the screen: what romeu wrote to the
// terminal, with the echo of what was typed, line ends as "\n". Once the
// screen holds prompt, it types answer; an empty prompt types nothing.
func (e *Env) RunTerminal(tb testing.TB, prompt, answer string, args []string, extra ...string) Result {
	tb.Helper()
	return e.runOnTerminal(tb, true, prompt, answer, args, extra...)
}

// RunStdinTerminal is Run with only the standard input of romeu on a
// terminal, and its standard output and error captured apart: the shell
// line romeu ... 2>log. It types nothing.
func (e *Env) RunStdinTerminal(tb testing.TB, args []string, extra ...string) Result {
	tb.Helper()
	return e.runOnTerminal(tb, false, "", "", args, extra...)
}

// runOnTerminal runs romeu with stdin on a terminal, and stderr too when
// stderrOnTerminal is set.
func (e *Env) runOnTerminal(tb testing.TB, stderrOnTerminal bool, prompt, answer string, args []string, extra ...string) Result {
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
	done := make(chan struct{})
	typed := make(chan error, 1)
	go func() {
		if prompt == "" {
			typed <- nil
			return
		}
		deadline := time.Now().Add(promptWait)
		for !strings.Contains(shown(), prompt) {
			select {
			case <-done:
				typed <- errors.New("romeu ended before it showed the prompt")
				return
			case <-time.After(5 * time.Millisecond):
			}
			if time.Now().After(deadline) {
				typed <- errors.New("the prompt never came")
				return
			}
		}
		_, err := io.WriteString(master, answer)
		typed <- err
	}()
	err := procgroup.Run(tb.Context(), procgroup.Command{
		Class:  procgroup.ToolStep,
		Argv:   append([]string{e.Romeu}, args...),
		Dir:    e.Home,
		Env:    e.Environ(extra...),
		Stdin:  terminal,
		Stdout: &stdout, Stderr: errOut,
	})
	close(done)
	terr := <-typed
	_ = terminal.Close()
	select {
	case <-drained:
	case <-time.After(promptWait):
		tb.Fatalf("romeu %v: the terminal never drained", args)
	}
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if stderrOnTerminal {
		res.Stderr = strings.ReplaceAll(screen.String(), "\r\n", "\n")
	}
	if terr != nil {
		tb.Fatalf("romeu %v: %v; stderr: %q", args, terr, res.Stderr)
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
