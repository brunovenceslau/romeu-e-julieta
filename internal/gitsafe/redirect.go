// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// ErrMoved marks the error of a clone or a fetch whose server answered
// the URL's first request with an HTTP redirect, which host mode does
// not follow (http.followRedirects=false, 05 5.1). Most often the
// repository has moved, and the fix is the operator's: the new URL in
// the spec or the settings. git does not say where the redirect points,
// and a redirect can have another cause (a login page, a URL without
// .git), so the error states only what git saw. A redirect of a later
// request of smart HTTP is refused too, and fails with git's own error.
//
// It is advice to the operator, read from git's standard error, which
// a server can write to: no decision may branch on it.
var ErrMoved = errors.New("gitsafe: the repository has moved or its server redirects")

// redirectStatus is the end of the line git writes, with LANG=C, when
// an HTTP request it made was answered with a redirect it does not
// follow: "fatal: unable to access '<url>/': " and this.
var redirectStatus = regexp.MustCompile(`The requested URL returned error: (30[12378])$`)

// redirectError is the error of a git whose standard error, kept whole,
// ends with redirectStatus: the error of the run, which it reads as,
// and the last line of that standard error.
type redirectError struct {
	last string
	err  error
}

// Error returns the error of the run.
func (e *redirectError) Error() string { return e.err.Error() }

// Unwrap returns the error of the run.
func (e *redirectError) Unwrap() error { return e.err }

// withRedirect returns err, the error of a git that failed, as a
// redirectError when stderr, its standard error, was kept whole and its
// last line ends with redirectStatus. git writes that line last; a
// server can make git echo text that holds the line (a pkt-line in
// "expected ACK/NAK, got '...'"), but git closes such an echo and goes
// on, so the line it forged is not the last.
func withRedirect(err error, stderr string, truncated bool) error {
	if truncated {
		return err
	}
	trimmed := strings.TrimRight(stderr, "\n")
	last := trimmed[strings.LastIndexByte(trimmed, '\n')+1:]
	if !redirectStatus.MatchString(last) {
		return err
	}
	return &redirectError{last: last, err: err}
}

// movedError returns ErrMoved, wrapped with url and the status, when
// err is a redirect of resolved, the URL git fetched for url, and err
// otherwise.
func movedError(err error, url, resolved string) error {
	var r *redirectError
	if !errors.As(err, &r) {
		return err
	}
	want := regexp.MustCompile(`^fatal: unable to access '` + regexp.QuoteMeta(strings.TrimSuffix(resolved, "/")) + `/?': ` + redirectStatus.String())
	m := want.FindStringSubmatch(r.last)
	if m == nil {
		return err
	}
	asked := url
	if resolved != url {
		// The repository's url.<base>.insteadOf rewrote it (05 5.1).
		asked = url + " (fetched as " + resolved + ")"
	}
	return fmt.Errorf("%w: %s answered HTTP %s, a redirect git does not follow here; if the repository has moved, set its new URL in the spec or the settings: %w",
		ErrMoved, termsafe.Escape(asked), m[1], err)
}
