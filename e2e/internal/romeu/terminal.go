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

// promptWait is how long RunTerminal waits for the prompt.
const promptWait = 30 * time.Second

// RunTerminal is Run with the standard input of romeu on the terminal of
// a /dev/ptmx pair (10 10.1, REQ-0790: a test helper over x/sys/unix, no
// pty dependency), and its standard output and error captured apart, so
// a test sees which stream a prompt went to. Once stderr holds prompt,
// it types answer on the terminal; an empty prompt types nothing.
func (e *Env) RunTerminal(tb testing.TB, prompt, answer string, args []string, extra ...string) Result {
	tb.Helper()
	master, terminal := openPTY(tb)
	// The terminal echoes what is typed to the master; draining it keeps
	// the echo from filling the terminal's output queue.
	go func() { _, _ = io.Copy(io.Discard, master) }()

	var stdout, stderr lockedBuffer
	done := make(chan struct{})
	typed := make(chan error, 1)
	go func() {
		if prompt == "" {
			typed <- nil
			return
		}
		deadline := time.Now().Add(promptWait)
		for !strings.Contains(stderr.String(), prompt) {
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
		Stdout: &stdout, Stderr: &stderr,
	})
	close(done)
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if terr := <-typed; terr != nil {
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
