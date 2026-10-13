// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package procgroup starts every subprocess of both binaries, of
// tools/ci and of tools/mutants (04 4.1, Subprocesses): each command
// runs in a process group of its own, with exactly the environment its
// caller gives and the timeout of its class (the timeout table of
// class.go); a timeout, a cancellation and the signal rule kill the
// whole group, and the drain of the output pipes after the kill is
// bounded by WaitDelay. Main is the one signal handler both mains
// install (04 4.1, Signals).
//
// A grandchild that calls setsid leaves the group, and the group kill
// does not reach it. Non-unix builds are unsupported on purpose: the
// binaries and the tools run on linux and darwin only.
package procgroup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// WaitDelay is how long Run lets the output pipes drain after the
// command is killed or exits (04 4.1, Subprocesses, item 4).
const WaitDelay = 2 * time.Second

// ErrNotFound is the error of a command whose program is not there; it
// is os/exec's, so errors.Is matches either name.
var ErrNotFound = exec.ErrNotFound

var (
	errNoArgv    = errors.New("procgroup: empty argv")
	errNoClass   = errors.New("procgroup: no timeout class")
	errNotAGroup = errors.New("procgroup: a pid at or below 1 names no group of ours")
)

// Command is one subprocess to run.
type Command struct {
	// Class is the row of the timeout table the command runs under.
	Class Class
	// Argv is the program and its arguments. The product names the
	// program by absolute path (04 4.1, Subprocesses, item 1); a bare
	// name is looked up in this process's PATH.
	Argv []string
	// Dir is the working directory; empty means this process's.
	Dir string
	// Env is the whole environment of the command, which never inherits
	// this process's: nil means an empty one.
	Env []string
	// Stdin is the standard input; nil means an empty one.
	Stdin io.Reader
	// Stdout and Stderr receive the output; nil discards it.
	Stdout io.Writer
	Stderr io.Writer
}

// ExitError is the error of a command that ended with a status other
// than 0, or that a signal ended.
type ExitError struct {
	Argv0  string         // the program that ran
	Code   int            // the exit status; -1 when a signal ended it
	Signal syscall.Signal // the signal that ended it; 0 when it exited
	err    error          // os/exec's error
}

// Error says how the program ended.
func (e *ExitError) Error() string {
	if e.Signal != 0 {
		return fmt.Sprintf("%s was ended by signal %d (%s)", e.Argv0, int(e.Signal), e.Signal)
	}
	return fmt.Sprintf("%s exited %d", e.Argv0, e.Code)
}

// Unwrap returns os/exec's error.
func (e *ExitError) Unwrap() error { return e.err }

// Run runs the command in a process group of its own and waits for it.
// The group is killed when ctx is cancelled, when the limit of the
// class passes, which is the error subprocess-timeout (*TimeoutError),
// when the signal handler of Main fires, and in every case once the
// leader has ended: os/exec kills only while the leader runs, so a
// grandchild that outlives a leader which ended, and holds the output
// pipe until WaitDelay, is still Run's to kill.
// A command that ends with a status other than 0 is an *ExitError.
func Run(ctx context.Context, c Command) error {
	if len(c.Argv) == 0 {
		return errNoArgv
	}
	if !c.Class.valid() {
		return fmt.Errorf("%w: %s", errNoClass, c.Argv[0])
	}
	runCtx := ctx
	limit := limitOf(ctx, c.Class)
	if limit > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, limit)
		defer cancel()
	}
	cmd := exec.CommandContext(runCtx, c.Argv[0], c.Argv[1:]...)
	cmd.Dir = c.Dir
	// os/exec gives a nil Env the environment of this process.
	cmd.Env = append([]string{}, c.Env...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = c.Stdin, c.Stdout, c.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return KillGroup(cmd.Process.Pid) }
	cmd.WaitDelay = WaitDelay
	g, _ := ctx.Value(groupsKey{}).(*groups)
	if err := g.start(cmd); err != nil {
		return err
	}
	err := cmd.Wait()
	g.end(cmd.Process.Pid)
	return result(ctx, runCtx, c, limit, err)
}

// result gives Run's error for what Wait returned.
func result(ctx, runCtx context.Context, c Command, limit time.Duration, err error) error {
	switch {
	case err == nil:
		return nil
	case ctx.Err() != nil:
		// The caller's cancellation or deadline, the signal rule's included.
		return fmt.Errorf("%s: %w: %w", c.Argv[0], context.Cause(ctx), err)
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		return &TimeoutError{Argv0: c.Argv[0], Class: c.Class, Limit: limit, err: fmt.Errorf("%w: %w", context.DeadlineExceeded, err)}
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		e := &ExitError{Argv0: c.Argv[0], Code: exit.ExitCode(), err: exit}
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			e.Signal = status.Signal()
		}
		return e
	}
	return err
}

// CombinedOutput runs the command as Run does and returns its standard
// output and standard error together, in the order it wrote them.
// Command's Stdout and Stderr are replaced.
func CombinedOutput(ctx context.Context, c Command) ([]byte, error) {
	var out bytes.Buffer
	c.Stdout, c.Stderr = &out, &out
	err := Run(ctx, c)
	return out.Bytes(), err
}

// KillGroup sends SIGKILL to the process group that pid leads. A group
// that is already gone is os.ErrProcessDone, which os/exec does not
// report from Cancel. A pid at or below 1 is refused: kill(-1)
// reaches every process this user may signal, and kill(0) the
// caller's own group.
func KillGroup(pid int) error {
	if pid <= 1 {
		return fmt.Errorf("%w: %d", errNotAGroup, pid)
	}
	err := syscall.Kill(-pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

// groupsKey is the context key of the groups Main's handler kills.
type groupsKey struct{}

// groups are the process groups that run under one Main, so that the
// handler of a signal kills them itself before it steps aside, rather
// than leave the kill to os/exec, which does it from a goroutine of its
// own after the cancel: a second signal in between would end this
// process by its default action and orphan the group (04 4.1, Signals,
// item 3, measured on 2026-10-12). A nil *groups is valid and tracks
// nothing: a Run outside Main.
type groups struct {
	mu       sync.Mutex
	leaders  map[int]bool // the pids of the leaders of the running groups
	isClosed bool         // the handler has fired: no group starts any more
}

// start starts cmd and records its group. The lock covers the start,
// so the handler finds either no group or the one started, and a start
// after the handler fired starts nothing.
func (g *groups) start(cmd *exec.Cmd) error {
	if g == nil {
		return cmd.Start()
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.isClosed {
		// The handler cancelled the context of Main before it closed.
		return fmt.Errorf("%s: %w", cmd.Path, context.Canceled)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if g.leaders == nil {
		g.leaders = map[int]bool{}
	}
	g.leaders[cmd.Process.Pid] = true
	return nil
}

// end kills the group of a leader that Wait reaped, and forgets it. A
// group with members keeps its pgid, so -pgid reaches only this group.
// The window after the group has fully emptied, between the last reap
// and this kill, which a reuse would need a full pid wrap to hit, is
// accepted, as it is on os/exec's Cancel path. The kill's own error is
// dropped on purpose: the command's result already stands, and
// WaitDelay has bounded the wait.
func (g *groups) end(pid int) {
	if g != nil {
		g.mu.Lock()
		defer g.mu.Unlock()
		delete(g.leaders, pid)
	}
	_ = KillGroup(pid)
}

// killAll kills every running group and lets no other start.
func (g *groups) killAll() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.isClosed = true
	for pid := range g.leaders {
		_ = KillGroup(pid)
	}
}

// interrupt is what the handler does with the first signal, in this
// order: cancel the command's context, kill every running group, then
// stop, which lets a second signal end the process by its default
// action, once no group is left to orphan.
func interrupt(cancel func(), g *groups, stop func()) {
	cancel()
	g.killAll()
	stop()
}
