// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

// The shapes of a credential helper, those of tools.gitCredentialHelper
// (03 3.4, REQ-1267): the name of a git-credential-<name> helper, or an
// absolute, clean path. Git runs a helper through sh (credential.c: a
// name becomes "git credential-<name> <op>", an absolute path
// "<path> <op>", a value that starts with "!" the shell snippet after
// it), so neither shape holds a character sh reads, and the "!" form is
// not one of them. A test holds the two to the rule of internal/spec on
// one corpus.
var (
	helperName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	helperPath = regexp.MustCompile(`^/[A-Za-z0-9._/+-]+$`)
)

// MaxCredentialHelper bounds a credential helper's value: PATH_MAX of
// macOS, the shorter of the two systems romeu runs on, so a longer path
// could not name a program anyway.
const MaxCredentialHelper = 1024

// CheckCredentialHelper returns ErrRefused, wrapped with the reason, for
// a value gitsafe does not hand to git as a credential helper.
func CheckCredentialHelper(helper string) error {
	switch {
	case len(helper) > MaxCredentialHelper:
		return fmt.Errorf("%w: a credential helper of %d bytes, over %d", ErrRefused, len(helper), MaxCredentialHelper)
	case helperName.MatchString(helper):
		return nil
	case helperPath.MatchString(helper) && path.Clean(helper) == helper:
		return nil
	}
	return fmt.Errorf("%w: the credential helper is neither a helper name nor an absolute, clean path without a shell character", ErrRefused)
}

// CheckCredentialHelperFile returns ErrRefused, wrapped with the rule
// that failed, for an absolute-path helper that someone other than the
// user or root could have written, since git runs it with the user's
// credentials in reach: it must be a regular, executable file, and it
// and every directory above it must be owned by the user or root and
// not writable by the group or others. A directory above the parent may
// be writable by others when it is root's and sticky, as /tmp is, since
// no one else can then rename or replace what the user put in it. A
// helper name is exempt: git finds git-credential-<name> in its own
// exec-path.
//
// The check resolves the symbolic links of the path and checks every
// element of the path both as given and as resolved, so a link to
// Homebrew's Cellar works, and no link, no directory a link passes
// through, and no directory the target sits in can be swapped by
// another user. It is a check of the time it runs: git runs the helper
// later, and a user's own files can change in between, which is the
// user's to control.
func CheckCredentialHelperFile(helper string) error {
	return checkHelperFile(helper, os.Getuid())
}

// checkHelperFile is CheckCredentialHelperFile for the user uid.
func checkHelperFile(helper string, uid int) error {
	if err := CheckCredentialHelper(helper); err != nil {
		return err
	}
	if helperName.MatchString(helper) {
		return nil
	}
	refuse := func(rule, p string) error {
		return fmt.Errorf("%w: the credential helper %s: %s", ErrRefused, p, rule)
	}
	real, err := filepath.EvalSymlinks(helper)
	if err != nil {
		why := err.Error()
		if pe, ok := errors.AsType[*fs.PathError](err); ok {
			why = pe.Err.Error() // the path is already in the message
		}
		return refuse("cannot be resolved: "+why, helper)
	}
	fi, err := os.Stat(real)
	switch {
	case err != nil:
		return refuse("cannot be read", real)
	case !fi.Mode().IsRegular():
		return refuse("is not a regular file", real)
	case fi.Mode().Perm()&0o111 == 0:
		return refuse("is not executable", real)
	}
	if why := ownership(fi, uid, false); why != "" {
		return refuse(why, real)
	}
	for _, p := range []string{helper, real} {
		for d, first := filepath.Dir(p), true; ; d, first = filepath.Dir(d), false {
			// The link itself, when an element is one: its owner can
			// point it elsewhere.
			if li, err := os.Lstat(d); err == nil && li.Mode()&fs.ModeSymlink != 0 {
				if why := ownership(li, uid, true); why != "" {
					return refuse(why, d)
				}
			}
			di, err := os.Stat(d)
			if err != nil {
				return refuse("cannot be read", d)
			}
			if why := ownership(di, uid, !first); why != "" {
				return refuse(why, d)
			}
			if d == "/" {
				break
			}
		}
	}
	return nil
}

// ownership returns the rule fi breaks, or "": owned by uid or root, and
// not writable by the group or others, but for a root-owned sticky
// directory when sticky is allowed, and for a symbolic link, whose mode
// git never reads.
func ownership(fi fs.FileInfo, uid int, sticky bool) string {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return "has no owner the system reports"
	}
	owner := int(st.Uid)
	switch {
	case owner != uid && owner != 0:
		return fmt.Sprintf("is owned by uid %d, neither the user nor root", owner)
	case fi.Mode()&fs.ModeSymlink != 0:
		return ""
	case fi.Mode().Perm()&0o022 == 0:
		return ""
	case sticky && fi.IsDir() && fi.Mode()&fs.ModeSticky != 0 && owner == 0:
		return ""
	}
	return "is writable by its group or others"
}

// CloneWithCredentialHelper is Clone with helper, a value
// CheckCredentialHelper admits, as the credential helper of the git
// clone command alone, for the one host that command connects to:
// credential.https://<host>.helper, set after the empty credential.helper
// of host mode, so this helper and no other answers that host, and no
// other host is ever handed to it. The host is that of the URL as git
// resolves it, the one git matches the key against. Host mode only; an
// empty helper is Clone.
//
// romeu init clones the config repo with it, before the host settings
// exist (04 4.2).
func (r *Runner) CloneWithCredentialHelper(ctx context.Context, url, dest, helper string) error {
	return r.clone(ctx, url, dest, helper)
}

// checkScopedHelper returns ErrRefused for a helper the runner does not
// hand to git: one of sandbox mode, or one CheckCredentialHelperFile
// refuses.
func (r *Runner) checkScopedHelper(helper string) error {
	if r.mode != Host {
		return fmt.Errorf("%w: a credential helper in %s mode", ErrRefused, r.mode)
	}
	return CheckCredentialHelperFile(helper)
}

// scopedHelper returns the setting that names helper for the host of
// the https URL raw only. Git matches credential.<url>.* by scheme, host
// and port, so the key carries the port when the URL does; a host
// hostPattern refuses would change the key git reads, and is refused.
func scopedHelper(raw, helper string) (setting, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || !hostPattern.MatchString(u.Host) {
		return setting{}, fmt.Errorf("%w: a credential helper for %q, which is not an https URL of a host", ErrRefused, raw)
	}
	return helperFor(u.Host, helper), nil
}

// helperFor is the setting that names helper for host only.
func helperFor(host, helper string) setting {
	return setting{"credential.https://" + strings.ToLower(host) + ".helper", helper}
}
