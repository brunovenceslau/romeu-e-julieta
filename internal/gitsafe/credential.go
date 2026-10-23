// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"
)

// maxHelper bounds a credential helper's value: PATH_MAX of macOS, the
// shorter of the two systems romeu runs on, so a longer path could not
// name a program anyway.
const maxHelper = 1024

// CloneWithCredentialHelper is Clone with helper, a git-credential-<name>
// helper's name or an absolute path (tools.gitCredentialHelper, 03 3.4),
// as the credential helper of the git clone command alone, for the one
// host that command connects to: credential.https://<host>.helper, set
// after the empty credential.helper of host mode, so this helper and no
// other answers that host, and no other host is ever handed to it. The
// host is that of the URL as git resolves it, the one git matches the
// key against. Host mode only; an empty helper is Clone.
//
// romeu init clones the config repo with it, before the host settings
// exist (04 4.2). Git runs a helper through sh (credential.c: a name
// becomes "git credential-<name> <op>", an absolute path "<path> <op>",
// a value starting with "!" the shell snippet after it), so the shapes
// admitted hold no character sh reads: the "!" form is refused, as is
// every value with a space, a quote, a $ or a newline.
func (r *Runner) CloneWithCredentialHelper(ctx context.Context, url, dest, helper string) error {
	return r.clone(ctx, url, dest, helper)
}

// checkScopedHelper returns ErrRefused for a helper the runner does not
// hand to git: one of sandbox mode, or one that is neither a helper name
// nor an absolute, clean path of the characters helperPath admits.
func (r *Runner) checkScopedHelper(helper string) error {
	switch {
	case r.mode != Host:
		return fmt.Errorf("%w: a credential helper in %s mode", ErrRefused, r.mode)
	case len(helper) > maxHelper:
		return fmt.Errorf("%w: a credential helper of %d bytes, over %d", ErrRefused, len(helper), maxHelper)
	case helperName.MatchString(helper):
		return nil
	case helperPath.MatchString(helper) && path.Clean(helper) == helper:
		return nil
	}
	return fmt.Errorf("%w: credential helper %q is neither a helper name nor an absolute, clean path", ErrRefused, helper)
}

// scopedHelper returns the setting that names helper for the host of
// the https URL raw only. Git matches credential.<url>.* by scheme, host
// and port, so the key carries the port when the URL does.
func scopedHelper(raw, helper string) (setting, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return setting{}, fmt.Errorf("%w: a credential helper for %q, which is not an https URL of a host", ErrRefused, raw)
	}
	return setting{"credential.https://" + strings.ToLower(u.Host) + ".helper", helper}, nil
}
