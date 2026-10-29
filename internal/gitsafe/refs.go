// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// fetchDestinations are the namespaces a fetch may write, by mode.
// Each is a prefix followed by a <dir> or <name> segment and at least
// one more component. On the host they are the three of 05 5.1, and
// never refs/heads, refs/tags, refs/remotes or HEAD (I5); in the
// sandbox, julieta's fetch of origin writes the remote-tracking refs.
var fetchDestinations = map[Mode][]string{
	Host:    {"refs/romeu/origin/", "refs/sandboxes/", "refs/romeu/snapshots/"},
	Sandbox: {"refs/remotes/"},
}

// createOnly are the namespaces CreateRef writes, by mode, with the
// same shape as fetchDestinations: the salvage refs and the base refs
// of a host clone (02 2.3), and the salvage refs of a sandbox clone
// (08 8.5). None is a fetch destination (I6).
var createOnly = map[Mode][]string{
	Host:    {"refs/romeu/salvage/", "refs/romeu/base/"},
	Sandbox: {"refs/salvage/"},
}

// objectID is a full SHA-1 or SHA-256 object id, in git's lower case.
var objectID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// checkObjectID returns ErrRefused for anything but a full object id.
func checkObjectID(oid string) error {
	if !objectID.MatchString(oid) {
		return fmt.Errorf("%w: %q is not a full object id", ErrRefused, oid)
	}
	return nil
}

// checkRefName returns ErrRefused, wrapped with the reason, for a name
// that is not a full ref name under refs/ by the rules of git
// check-ref-format; with pattern, one * is allowed, as in a refspec
// side. gitsafe is stricter than git in two ways: the name must start
// with refs/, and it must be valid UTF-8.
func checkRefName(name string, pattern bool) error {
	refuse := func(why string) error {
		return fmt.Errorf("%w: ref name %q %s", ErrRefused, name, why)
	}
	switch {
	case !strings.HasPrefix(name, "refs/"):
		return refuse("does not start with refs/")
	case !utf8.ValidString(name):
		return refuse("is not valid UTF-8")
	case strings.Contains(name, ".."):
		return refuse("holds ..")
	case strings.Contains(name, "@{"):
		return refuse("holds @{")
	case strings.HasSuffix(name, "."):
		return refuse("ends with a dot")
	case strings.ContainsFunc(name, func(r rune) bool { return isControl(r) || strings.ContainsRune(" ~^:?[\\", r) }):
		return refuse("holds a character git refuses")
	}
	stars := strings.Count(name, "*")
	if stars > 1 || (stars == 1 && !pattern) {
		return refuse("holds a * where none is allowed")
	}
	for c := range strings.SplitSeq(name, "/") {
		switch {
		case c == "":
			return refuse("has an empty component")
		case strings.HasPrefix(c, "."):
			return refuse("has a component that starts with a dot")
		case strings.HasSuffix(c, ".lock"):
			return refuse("has a component that ends with .lock")
		}
	}
	return nil
}

// originPrefix is the namespace of the origin fetches of a host clone,
// one directory per repo dir (02 2.3).
const originPrefix = "refs/romeu/origin/"

// remoteHEAD is the one refspec source that is not a ref name under
// refs/: the remote's HEAD, which a host fetch takes into
// refs/romeu/origin/<dir>/HEAD only, so a repo with no ref resolves
// its egress from the origin (I9; decided on 2026-10-29, T183).
const remoteHEAD = "HEAD"

// checkRefspec returns ErrRefused, wrapped with the reason, for a
// fetch refspec of the mode that is not <src>:<dst>, with an optional
// leading +, both sides full ref names under refs/, a * on both sides
// or on neither, and <dst> in a fetch destination of the mode (I5).
// One source is not under refs/: on the host, HEAD, the remote's HEAD,
// into refs/romeu/origin/<dir>/HEAD only, and no other source writes
// that ref, so refs/romeu/origin/<dir>/HEAD is written only by a fetch
// whose source is HEAD (decided on 2026-10-29, T183). A pattern under
// refs/romeu/origin/ has its * as the whole last component of its
// destination, so only a source ref whose * part is HEAD maps onto a
// HEAD, which Fetch excludes (originNegatives).
//
//romeu:invariant I5 guard
func checkRefspec(mode Mode, spec string) error {
	src, dst, ok := splitRefspec(spec)
	if !ok {
		return fmt.Errorf("%w: refspec %q has no destination", ErrRefused, spec)
	}
	intoHEAD := mode == Host && isOriginHEAD(dst)
	switch {
	case src == remoteHEAD && (!intoHEAD || !strings.HasSuffix(dst, "/"+remoteHEAD)):
		return fmt.Errorf("%w: refspec %q fetches the remote's HEAD outside %s<dir>/HEAD", ErrRefused, spec, originPrefix)
	case src != remoteHEAD && intoHEAD:
		return fmt.Errorf("%w: refspec %q writes %s<dir>/HEAD from another source than the remote's HEAD", ErrRefused, spec, originPrefix)
	case src != remoteHEAD:
		if err := checkRefName(src, true); err != nil {
			return fmt.Errorf("refspec %q: %w", spec, err)
		}
	}
	if err := checkRefName(dst, true); err != nil {
		return fmt.Errorf("refspec %q: %w", spec, err)
	}
	if strings.Contains(src, "*") != strings.Contains(dst, "*") {
		return fmt.Errorf("%w: refspec %q has a * on one side only", ErrRefused, spec)
	}
	if strings.HasPrefix(dst, originPrefix) && strings.Contains(dst, "*") && !strings.HasSuffix(dst, "/*") {
		return fmt.Errorf("%w: refspec %q has a * that is not the whole last component of its destination under %s", ErrRefused, spec, originPrefix)
	}
	if !inNamespace(dst, fetchDestinations[mode]) {
		return fmt.Errorf("%w: refspec %q writes outside %s", ErrRefused, spec, strings.Join(fetchDestinations[mode], ", "))
	}
	return nil
}

// splitRefspec returns the source and destination of a fetch refspec,
// its leading + dropped, and false when it has no colon.
func splitRefspec(spec string) (src, dst string, ok bool) {
	return strings.Cut(strings.TrimPrefix(spec, "+"), ":")
}

// isOriginHEAD reports whether ref is refs/romeu/origin/<dir>/<name>,
// <dir> one segment with no *, and <name> HEAD under case folding: on
// a case-insensitive file system, git stores <dir>/head as the same
// file as <dir>/HEAD.
func isOriginHEAD(ref string) bool {
	rest, ok := strings.CutPrefix(ref, originPrefix)
	dir, name, cut := strings.Cut(rest, "/")
	return ok && cut && dir != "" && !strings.Contains(dir, "*") && strings.EqualFold(name, remoteHEAD)
}

// inNamespace reports whether ref, a name checkRefName accepts, is in
// one of the namespaces: a prefix, a segment with no *, and at least
// one more component.
func inNamespace(ref string, namespaces []string) bool {
	for _, ns := range namespaces {
		rest, ok := strings.CutPrefix(ref, ns)
		if !ok {
			continue
		}
		segment, tail, ok := strings.Cut(rest, "/")
		if ok && segment != "" && !strings.Contains(segment, "*") && tail != "" {
			return true
		}
	}
	return false
}

// CreateRef creates ref at oid in the repository in dir, and fails when
// ref already exists, whatever it points to (I6). ref must be in a
// create-only namespace of the mode: a salvage or base ref.
//
//romeu:invariant I6 guard
func (r *Runner) CreateRef(ctx context.Context, dir, ref, oid string) error {
	if err := checkRepo(dir); err != nil {
		return err
	}
	if err := checkRefName(ref, false); err != nil {
		return err
	}
	if !inNamespace(ref, createOnly[r.mode]) {
		return fmt.Errorf("%w: ref %q is outside %s", ErrRefused, ref, strings.Join(createOnly[r.mode], ", "))
	}
	if err := checkObjectID(oid); err != nil {
		return err
	}
	// An empty old value makes update-ref fail when the ref exists.
	return r.run(ctx, dir, nil, "update-ref", "--no-deref", ref, oid, "")
}
