// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import "fmt"

// The error git-floor of 04 4.4, the one error id this package
// declares: its id, its slug, its exit code and its hint.
const (
	floorID   = "RJ-344"
	floorSlug = "git-floor"
	floorExit = 2
	floorFix  = "decide: install git at or above the floor of 05 5.1"
)

// FloorError is the error git-floor (RJ-344): the host git is below
// the floor of 05 5.1, or its version could not be read.
type FloorError struct {
	Found string // the version git reported, or its output when unreadable
	err   error  // why the output could not be read; nil when it was
}

// Error says which git is below which floor.
func (e *FloorError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("git version %q cannot be read, so it is below the floor %s: %v", e.Found, Floor(), e.err)
	}
	return fmt.Sprintf("git %s is below the floor %s", e.Found, Floor())
}

// Unwrap returns why the version could not be read, or nil.
func (e *FloorError) Unwrap() error { return e.err }

// ID returns the stable id of the error, RJ-344.
func (e *FloorError) ID() string { return floorID }

// Slug returns the readable name of the error, git-floor.
func (e *FloorError) Slug() string { return floorSlug }

// ExitCode returns the exit status of a command that fails with it: 2.
func (e *FloorError) ExitCode() int { return floorExit }

// Details names the version found and the floor.
func (e *FloorError) Details() string {
	return fmt.Sprintf("found %s, floor %s", e.Found, Floor())
}

// Fix is the hint of 04 4.4.
func (e *FloorError) Fix() string { return floorFix }
