// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import "context"

// Version runs git version and returns the version git reports, as
// major.minor.patch with ".rc" for a release candidate, the form the
// floor of 05 5.1 compares; a build suffix such as " (Apple Git-154)" is
// left out. An output that is not a git version line is the error
// git-floor (*FloorError, RJ-344), as CheckFloor says of it: a version
// gitsafe cannot read is not one it can trust. A git that does not run
// is the error of the run. romeu version --json reports it in its
// versions object (04 4.1, REQ-0286).
func (r *Runner) Version(ctx context.Context) (string, error) {
	v, err := r.readVersion(ctx)
	if err != nil {
		return "", err
	}
	return v.String(), nil
}

// readVersion runs git version, with its output bounded as every read
// of gitsafe is, and parses its first line strictly: an output
// parseVersion refuses is a *FloorError that quotes it, escaped.
func (r *Runner) readVersion(ctx context.Context) (version, error) {
	out, err := r.output(ctx, "", nil, "version")
	if err != nil {
		return version{}, err
	}
	v, err := parseVersion(out)
	if err != nil {
		return version{}, &FloorError{Found: quoteVersion(out), err: err}
	}
	return v, nil
}
