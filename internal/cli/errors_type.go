// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"errors"
	"maps"
	"slices"
	"strings"
)

// Row is one row of the error table (04 4.1, Error ids; 04 4.4): the
// stable id RJ-<nnn>, the slug that is its readable name, the exit code
// a command that fails with it returns, the fix hint, and the "recovered
// in" cell, a docs/guide/ anchor or "none: <reason>". Each package
// declares its rows in its own errors.go, as package-level variables
// whose fields are literals, so one test reads the union from the source
// (ReadTable) and the generator writes the reference pages from it.
type Row struct {
	ID          string
	Slug        string
	Exit        int
	Fix         string
	RecoveredIn string
}

// Detail is one line of the details of an error: the text form prints
// it as "<key>: <value>" after the first line and the fix, and the
// --json form carries it in the details object.
type Detail struct {
	Key   string
	Value string
}

// Error is an error of the table: the row it was raised as, the message
// of this occurrence, its details, and the error it wraps, if any. A
// command returns one, or wraps one with %w; Main prints the outermost
// one that errors.As finds and exits with its row's code.
type Error struct {
	Row
	Message string
	Details []Detail
	// Fill holds the values of the placeholders of the row's fix hint,
	// such as "<binary>", from the context of this occurrence.
	Fill map[string]string
	Err  error
}

// New returns an error of row r with the message and details given.
func (r Row) New(message string, details ...Detail) *Error {
	return &Error{Row: r, Message: message, Details: details}
}

// Wrap returns an error of row r with the message and details given,
// which wraps err. Report prints err's text after the message as it is,
// in the text and the --json forms, so a caller wraps only an error
// whose text holds no secret value (04 4.1, Error ids, item 8) and puts
// what it cannot vouch for in a detail it has cut and cleaned.
func (r Row) Wrap(err error, message string, details ...Detail) *Error {
	return &Error{Row: r, Message: message, Details: details, Err: err}
}

// Error returns the message, then the wrapped error's text, if any.
func (e *Error) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return e.Message + ": " + e.Err.Error()
}

// Unwrap returns the wrapped error, or nil.
func (e *Error) Unwrap() error { return e.Err }

// Hint returns the fix hint of the row with its placeholders filled
// from Fill.
func (e *Error) Hint() string {
	keys := slices.Sorted(maps.Keys(e.Fill))
	pairs := make([]string, 0, 2*len(keys))
	for _, key := range keys {
		pairs = append(pairs, key, e.Fill[key])
	}
	return strings.NewReplacer(pairs...).Replace(e.Fix)
}

// Of returns the error of the table that err carries: the outermost
// *Error that errors.As finds in its chain. A chain with none is a
// defect of the code, which Of returns as the error internal (RJ-348),
// wrapping err; so is a nil *Error and a row whose exit code is not an
// error's. It returns nil for a nil err.
func Of(err error) *Error {
	if err == nil {
		return nil
	}
	if e, ok := errors.AsType[*Error](err); ok && e != nil && e.Exit > 0 {
		return e
	}
	return Internal.Wrap(err, "an error with no id of the error table")
}

// ExitCode returns the exit status of a command that returned err: 0
// for nil, otherwise the exit code of the row of Of(err).
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	return Of(err).Exit
}
