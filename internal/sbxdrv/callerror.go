// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// StderrHead is how much of the standard error of a failed call enters
// its details (04 4.1, Output, REQ-0270).
const StderrHead = 4 << 10

// Versions are the versions the details of a failed call carry
// (04 4.1, Error ids, item 7); a version the caller does not know is
// left empty and printed as unknown.
type Versions struct {
	Romeu   string
	Julieta string
	Sbx     string
}

// CallError is the error of an sbx call that did not succeed: sbx
// exited with a status other than 0 or a signal ended it, it could not
// be started, or it ran past its timeout. It wraps what procgroup
// returned, so errors.As finds a *procgroup.TimeoutError
// (subprocess-timeout, RJ-323) in it.
//
// It is not a row of the error table of its own: the command that made
// the call decides which row the failure is, such as sbx-unknown for sbx
// ls, and hands Details to it.
type CallError struct {
	// Call is the call that failed.
	Call Call
	// Err is what procgroup returned.
	Err error

	argv     string   // the escaped argv
	status   string   // how the call ended
	stderr   string   // the head of the stderr, with the host's names replaced, escaped
	step     string   // the numbered step of 04 4.2 the call ran in
	versions Versions // the versions of the report
}

// Error says which call failed and how, with no output of sbx in it.
func (e *CallError) Error() string {
	return fmt.Sprintf("sbx %s failed: %s", e.Call.Name, e.status)
}

// Unwrap returns what procgroup returned.
func (e *CallError) Unwrap() error { return e.Err }

// Details are the details of 04 4.1, Error ids, item 7, in the order
// the text form prints them: the escaped argv, the exit status, the
// head of the stderr, the step and the three versions. The argv, the
// status, the step and the versions hold no secret value (I25).
func (e *CallError) Details() []cli.Detail {
	unknown := func(v string) string {
		if v == "" {
			return "unknown"
		}
		return v
	}
	d := []cli.Detail{
		{Key: "argv", Value: e.argv},
		{Key: "status", Value: e.status},
		{Key: "stderr", Value: e.stderr},
	}
	if e.step != "" {
		d = append(d, cli.Detail{Key: "step", Value: e.step})
	}
	return append(d,
		cli.Detail{Key: "romeu version", Value: unknown(e.versions.Romeu)},
		cli.Detail{Key: "julieta version", Value: unknown(e.versions.Julieta)},
		cli.Detail{Key: "sbx version", Value: unknown(e.versions.Sbx)})
}

// newCallError builds the error of a call that did not succeed, from
// what procgroup returned and the stderr it captured.
func newCallError(c Call, err error, stderr []byte, step string, v Versions, s Scrubber) *CallError {
	return &CallError{
		Call:     c,
		Err:      err,
		argv:     escapedArgv(c.Argv),
		status:   statusOf(err),
		stderr:   head(stderr, s),
		step:     step,
		versions: v,
	}
}

// statusOf says how the call ended, from the error procgroup returned.
func statusOf(err error) string {
	if te, ok := errors.AsType[*procgroup.TimeoutError](err); ok {
		return "timed out: " + te.Details()
	}
	if ee, ok := errors.AsType[*procgroup.ExitError](err); ok {
		if ee.Signal != 0 {
			return "ended by signal " + strconv.Itoa(int(ee.Signal)) + " (" + signalName(ee.Signal) + ")"
		}
		return "exit " + strconv.Itoa(ee.Code)
	}
	if errors.Is(err, procgroup.ErrNotFound) {
		return "sbx was not found"
	}
	return "did not complete"
}

// signalName is the name of a signal, as the syscall package prints it.
func signalName(s syscall.Signal) string { return s.String() }

// escapedArgv renders the argv for the details: sbx, then each word
// escaped by termsafe, and quoted when it is empty or holds a space or a
// quotation mark.
func escapedArgv(argv []string) string {
	words := make([]string, 0, len(argv)+1)
	words = append(words, "sbx")
	for _, w := range argv {
		w = termsafe.Escape(w)
		if w == "" || strings.ContainsAny(w, " \"'") {
			w = strconv.Quote(w)
		}
		words = append(words, w)
	}
	return strings.Join(words, " ")
}

// head returns the first StderrHead bytes of stderr, with the host's
// names replaced before the cut, so that a name cut in two does not
// leave half of it behind, and escaped by termsafe. The cut falls on
// the start of a character.
func head(stderr []byte, s Scrubber) string {
	text := s.Replace(string(stderr))
	if len(text) > StderrHead {
		i := StderrHead
		for i > 0 && !utf8.RuneStart(text[i]) {
			i--
		}
		text = text[:i]
	}
	return termsafe.Escape(text)
}
