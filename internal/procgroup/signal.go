// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

// signalBase is added to the number of the signal that ended a command,
// for its exit status: 130 for SIGINT, 143 for SIGTERM.
const signalBase = 128

// beforeStepAside runs in the handler right before it steps aside, with
// the channel the handler's signals arrive on. It is nil but in the
// helper processes of tests, which set it before Main to hold the
// handler at that point.
var beforeStepAside func(sigs <-chan os.Signal)

// SetBeforeStepAside sets the function Main's handler runs right before
// it steps aside, and returns the one it replaces. It is for the helper
// processes of the tests of a main that installs Main, which send a
// second signal there and read it from sigs, so that it has landed
// before the handler steps aside whatever the scheduling: a signal a
// process sends itself may arrive after kill returns, as on macOS. The
// handler waits for f, so f must return. Test only: the call-site scan
// of this package fails a call outside a _test.go file. Call it before
// Main.
func SetBeforeStepAside(f func(sigs <-chan os.Signal)) (previous func(sigs <-chan os.Signal)) {
	previous, beforeStepAside = beforeStepAside, f
	return previous
}

// Main runs the command under the signal rule of 04 4.1, Signals, and
// returns its exit status, for os.Exit. Both binaries, tools/ci and
// tools/mutants install it in main, and nothing else maps a signal.
//
// The first SIGINT or SIGTERM cancels the context run gets, kills the
// group of every command that runs under it (Run), then lets the next
// signal end the process by its default action. In that order, a
// second signal finds no group to orphan. When run returns after the
// signal, an interrupted command says so in one line, "<name>:
// interrupted", and exits with 128 plus the signal number, even when it
// had already found a failure (1 to 6): the signal wins. A command
// that had finished its work keeps 0.
func Main(name string, stderr io.Writer, run func(ctx context.Context) int) int {
	g := &groups{}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), groupsKey{}, g))
	defer cancel()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)
	done := make(chan struct{})
	handled := make(chan syscall.Signal, 1)
	go func() {
		defer close(handled)
		select {
		case sig := <-sigs:
			// Notify delivers only the two signals above, both a
			// syscall.Signal on unix.
			s, _ := sig.(syscall.Signal)
			interrupt(cancel, g, func() {
				if beforeStepAside != nil {
					beforeStepAside(sigs)
				}
				signal.Stop(sigs)
			})
			// Sent once the groups are dead, so Main returns no status
			// while a group of its command still runs.
			handled <- s
		case <-done:
		}
	}()
	status := run(ctx)
	close(done)
	// The handler has either taken a signal, whose number comes first,
	// or taken none and closed the channel, which reads as 0.
	sig := <-handled
	if sig == 0 || status == 0 {
		return status
	}
	_, _ = fmt.Fprintf(stderr, "%s: interrupted\n", name)
	return signalBase + int(sig)
}
