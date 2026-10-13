// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

import (
	"fmt"
	"time"
)

// The error subprocess-timeout of 04 4.4, the one error id this package
// declares: its id, its slug and its exit code.
const (
	timeoutID   = "RJ-323"
	timeoutSlug = "subprocess-timeout"
	timeoutExit = 1
)

// TimeoutError is the error subprocess-timeout (RJ-323): a command ran
// past the limit of its class, and its group was killed. It wraps
// context.DeadlineExceeded.
type TimeoutError struct {
	Argv0 string        // the program that ran
	Class Class         // the class whose limit it passed
	Limit time.Duration // that limit
	err   error         // what Run saw: the deadline, and the command's own error
}

// Error says which program passed which limit.
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("%s ran past the %s limit of class %s", e.Argv0, e.Limit, e.Class)
}

// Unwrap returns what Run saw, which holds context.DeadlineExceeded.
func (e *TimeoutError) Unwrap() error { return e.err }

// ID returns the stable id of the error, RJ-323.
func (e *TimeoutError) ID() string { return timeoutID }

// Slug returns the readable name of the error, subprocess-timeout.
func (e *TimeoutError) Slug() string { return timeoutSlug }

// ExitCode returns the exit status of a command that fails with it: 1,
// the operation failed.
func (e *TimeoutError) ExitCode() int { return timeoutExit }

// Details names the class and its limit (04 4.1, Subprocesses, item 5).
func (e *TimeoutError) Details() string {
	return fmt.Sprintf("class %s, limit %s", e.Class, e.Limit)
}

// Fix is the hint of the error: the program may still be working, and
// sbx ls --json shows whether a sandbox or the daemon is.
func (e *TimeoutError) Fix() string {
	return "the sandbox or the sbx daemon may still be busy: `sbx ls --json` shows its state; run the command again once it is idle"
}
