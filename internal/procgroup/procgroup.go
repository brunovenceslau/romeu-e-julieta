// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package procgroup starts every subprocess of both binaries, of
// tools/ci and of tools/mutants (04 4.1, Subprocesses): each command
// runs in a process group of its own, with the timeout of its class
// (the timeout table of class.go); a timeout, a cancellation and the
// signal rule kill the whole group, and so does the end of the leader,
// and the drain of the output pipes after the kill is bounded by
// WaitDelay. Main is the one signal handler both mains install (04 4.1,
// Signals).
//
// The scrubbed environment of 04 4.1 has its one implementation here
// in this form: a command gets exactly the Env its caller gives and
// never inherits this process's. Which variables a call carries, and
// that the product names sbx and git by absolute path from host
// settings (04 4.1, Subprocesses, item 1), are the callers' rules:
// sbxdrv and gitsafe for romeu, the mise, hooks and layout packages for
// julieta, and tools/ci and tools/mutants for themselves.
//
// A grandchild that calls setsid, or setpgid into a group of its own,
// leaves the group, and the group kill does not reach it. Non-unix builds are unsupported on purpose: the
// binaries and the tools run on linux and darwin only.
package procgroup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// WaitDelay is how long Run lets the output pipes drain after the
// group is killed (04 4.1, Subprocesses, item 4).
const WaitDelay = 2 * time.Second

// ErrNotFound is the error of a command whose program is not there,
// named on PATH or by a path. It is os/exec's, so errors.Is matches
// either name.
var ErrNotFound = exec.ErrNotFound

var (
	errNoArgv    = errors.New("procgroup: empty argv")
	errNoClass   = errors.New("procgroup: no timeout class")
	errNotAGroup = errors.New("procgroup: not a group of ours to kill")
)

// kill is syscall.Kill. Tests replace it with a recorder, so that no
// test sends a real signal to a pid KillGroup must refuse; a test that
// swaps it must not run in parallel with any other test of this
// package.
var kill = syscall.Kill

// Command is one subprocess to run.
type Command struct {
	// Class is the row of the timeout table the command runs under.
	Class Class
	// Argv is the program and its arguments. A bare name is looked up
	// in this process's PATH.
	Argv []string
	// Dir is the working directory; empty means this process's.
	Dir string
	// Env is the whole environment of the command, which never inherits
	// this process's: nil means an empty one.
	Env []string
	// Stdin is the standard input; nil means an empty one. A reader
	// that is not a file is copied through a pipe Run closes when the
	// leader ends; a copy blocked in the reader itself ends only when
	// the reader returns, and Run does not wait for it.
	Stdin io.Reader
	// Stdout and Stderr receive the output; nil discards it. When both
	// are the same writer, the command writes both to one pipe, in the
	// order it wrote them. A writer must not block: while it does, the
	// command blocks on the full pipe, and only its timeout or a
	// cancellation ends it. A writer that fails stops the copy and
	// closes the pipe, so the command gets EPIPE or SIGPIPE.
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
//
// The group is killed when ctx is cancelled, when the limit of the
// class passes, which is the error subprocess-timeout (*TimeoutError),
// when the signal handler of Main fires, and in every case as soon as
// the leader has ended: os/exec kills only while the leader runs, so a
// grandchild that outlives the leader is Run's to kill. Run reads the
// output through pipes of its own, so it can kill the group between the
// end of the leader and the drain: a grandchild in the group that holds
// the output pipe dies at once, and the pipe closes.
//
// After the kill the drain waits at most WaitDelay, for a holder of the
// pipe that left the group with setsid. Past that bound Run stops
// reading and returns the leader's result, nil for a leader that
// exited 0: the command's own work succeeded, its group is dead, and
// what it wrote before it ended has been read; only what the escaped
// holder writes later is lost, and it was never the command's result.
//
// A command that ends with a status other than 0 is an *ExitError, and
// a program that is not there is ErrNotFound.
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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return KillGroup(cmd.Process.Pid) }
	cmd.WaitDelay = WaitDelay
	out, err := attach(cmd, c.Stdin, c.Stdout, c.Stderr)
	if err != nil {
		return err
	}
	g, _ := ctx.Value(groupsKey{}).(*groups)
	if err := g.start(cmd); err != nil {
		out.abort()
		return startError(c, err)
	}
	out.started()
	err = cmd.Wait()
	out.leaderEnded()
	g.end(cmd.Process.Pid)
	if drainErr := out.drain(WaitDelay); err == nil {
		err = drainErr
	}
	return result(ctx, runCtx, c, limit, err)
}

// startError gives Run's error for a command that did not start. The
// kernel says ENOENT both for a missing program and for a missing
// working directory, so a program named by a path is looked up to tell
// the two apart.
func startError(c Command, err error) error {
	if errors.Is(err, exec.ErrNotFound) || !errors.Is(err, fs.ErrNotExist) || !strings.Contains(c.Argv[0], "/") {
		return err
	}
	program := c.Argv[0]
	if !filepath.IsAbs(program) && c.Dir != "" {
		program = filepath.Join(c.Dir, program)
	}
	if _, statErr := os.Stat(program); errors.Is(statErr, fs.ErrNotExist) {
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	return err
}

// result gives Run's error for what Wait returned. A command that
// failed at the moment its limit passed is reported as the timeout:
// Wait cannot tell a failure the deadline caused from one it did not.
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
// Command's Stdout and Stderr are replaced. The output is held in
// memory with no cap, so a command of a class with no timeout, or one
// whose output can be large, streams to a writer through Run instead.
func CombinedOutput(ctx context.Context, c Command) ([]byte, error) {
	var out bytes.Buffer
	c.Stdout, c.Stderr = &out, &out
	err := Run(ctx, c)
	return out.Bytes(), err
}

// KillGroup sends SIGKILL to the process group that pid leads. A group
// that is already gone is os.ErrProcessDone, which os/exec does not
// report from Cancel. It refuses a pid that names no group of a child:
// at or below 1, since kill(-1) reaches every process this user may
// signal and kill(0) the caller's own group, and the caller's own
// group, which a child in a group of its own never leads.
func KillGroup(pid int) error {
	if pid <= 1 || pid == syscall.Getpgrp() {
		return fmt.Errorf("%w: %d", errNotAGroup, pid)
	}
	err := kill(-pid, syscall.SIGKILL)
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
	leaders  map[int]struct{} // the pids of the leaders of the running groups
	isClosed bool             // the handler has fired: no group starts any more
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
		g.leaders = map[int]struct{}{}
	}
	g.leaders[cmd.Process.Pid] = struct{}{}
	return nil
}

// end kills the group of a leader that Wait reaped, and forgets it.
// os/exec signals a leader through a pidfd on Linux, but a group has
// none, so the kill names the group by number. While the group has a
// member its pgid is not reused, so -pgid reaches only this group; the
// window after it has fully emptied, between the last reap and this
// kill, would need a full wrap of the pid space to hit, and is
// accepted. The kill's own error is dropped on purpose: the command's
// result already stands, and the drain is bounded.
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
