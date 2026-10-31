// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// Version runs git version and returns the version git reports, its
// own token: the field after "git version " up to the first space, such
// as 2.53.0, 2.46.0.rc1 or 2.45.4.windows.1, so a report pastes what git
// prints; a suffix after the space, such as " (Apple Git-154)", is left
// out. The token is returned only once its line passes the strict parse
// the floor of 05 5.1 compares, and it is cut to foundLimit bytes and
// escaped with termsafe. An output that is not a git version line is
// the error git-floor (*FloorError, RJ-344), as CheckFloor says of it: a
// version gitsafe cannot read is not one it can trust. A git that does
// not run is the error of the run. romeu version --json reports it in
// its versions object (04 4.1, REQ-0286).
func (r *Runner) Version(ctx context.Context) (string, error) {
	_, token, err := r.readVersion(ctx)
	if err != nil {
		return "", err
	}
	return token, nil
}

// readVersion runs git version, with its output bounded as every read
// of gitsafe is, and parses its first line strictly: an output
// parseVersion refuses is a *FloorError that quotes it, escaped. It
// returns the parsed version, which the floor compares, and git's own
// token of it (versionToken).
func (r *Runner) readVersion(ctx context.Context) (version, string, error) {
	out, err := r.output(ctx, "", nil, "version")
	if err != nil {
		return version{}, "", err
	}
	v, err := parseVersion(out)
	if err != nil {
		return version{}, "", &FloorError{Found: quoteVersion(out), err: err}
	}
	return v, versionToken(out), nil
}

// versionToken returns the field of git's version line after "git
// version ", up to the first space, cut to foundLimit bytes and escaped
// with termsafe; out is a line parseVersion read.
func versionToken(out string) string {
	line, _, _ := strings.Cut(out, "\n")
	token, _ := strings.CutPrefix(line, "git version ")
	token, _, _ = strings.Cut(token, " ")
	if len(token) > foundLimit {
		token = token[:foundLimit]
	}
	return termsafe.Escape(token)
}
