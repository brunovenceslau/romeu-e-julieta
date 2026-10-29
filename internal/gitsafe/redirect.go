// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrMoved marks the error of a clone or a fetch whose server answered
// the URL with an HTTP redirect, which host mode does not follow
// (http.followRedirects=false, 05 5.1). Most often the repository has
// moved, and the fix is the operator's, the new URL in the spec or the
// settings; git does not say where the redirect points, and a redirect
// can have another cause (a login page, a URL without .git), so the
// error states only what git saw.
var ErrMoved = errors.New("gitsafe: the repository has moved or its server redirects")

// redirectLine is the line git writes, with LANG=C, when an HTTP
// request it made was answered with a redirect it does not follow: the
// URL it asked for and the status. A server writes only lines git
// prefixes with "remote: ", so it cannot write this one.
var redirectLine = regexp.MustCompile(`(?m)^fatal: unable to access '([^'\n]*)': The requested URL returned error: (30[12378])$`)

// redirectError is the error of a git that stopped at a redirect: the
// error of the run, which it reads as, and the URL and status of
// redirectLine.
type redirectError struct {
	url, status string
	err         error
}

// Error returns the error of the run.
func (e *redirectError) Error() string { return e.err.Error() }

// Unwrap returns the error of the run.
func (e *redirectError) Unwrap() error { return e.err }

// withRedirect returns err, the error of a git that failed, as a
// redirectError when its standard error holds redirectLine.
func withRedirect(err error, stderr string) error {
	m := redirectLine.FindStringSubmatch(stderr)
	if m == nil {
		return err
	}
	return &redirectError{url: m[1], status: m[2], err: err}
}

// movedError returns ErrMoved, wrapped with url and the status, when
// err is a redirect of resolved, the URL git fetched for url, and err
// otherwise.
func movedError(err error, url, resolved string) error {
	var r *redirectError
	if !errors.As(err, &r) || strings.TrimSuffix(r.url, "/") != strings.TrimSuffix(resolved, "/") {
		return err
	}
	asked := url
	if resolved != url {
		// The repository's url.<base>.insteadOf rewrote it (05 5.1).
		asked = url + " (fetched as " + resolved + ")"
	}
	return fmt.Errorf("%w: %s answered HTTP %s, a redirect gitsafe does not follow; if the repository has moved, set its new URL in the spec or the settings: %w",
		ErrMoved, asked, r.status, err)
}
