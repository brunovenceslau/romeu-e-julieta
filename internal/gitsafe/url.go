// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// loopback is the one host a git:// URL may name: the sandbox daemon
// on the host's loopback (05 5.1).
const loopback = "127.0.0.1"

// CheckURL returns ErrRefused, wrapped with the reason, for a URL
// gitsafe never fetches from (05 5.1): it accepts https://<host>/...
// and git://127.0.0.1[:port]/... only, so file:// URLs, plain paths,
// git:// on any other host and every other transport are refused. A
// URL with user information is refused too, since a credential in a
// URL would reach git's argv. There is no test override: tests serve
// origins over HTTPS or on the loopback daemon.
func CheckURL(raw string) error {
	if strings.ContainsFunc(raw, func(r rune) bool { return isControl(r) || r == ' ' }) {
		return fmt.Errorf("%w: URL %q holds a space or a control character", ErrRefused, raw)
	}
	scheme, _, ok := strings.Cut(raw, "://")
	if !ok || (scheme != "https" && scheme != "git") {
		return fmt.Errorf("%w: URL %q is neither https:// nor git://", ErrRefused, raw)
	}
	u, err := url.Parse(raw)
	switch {
	case err != nil:
		return fmt.Errorf("%w: URL %q: %w", ErrRefused, raw, err)
	case u.Host == "" || u.Opaque != "":
		return fmt.Errorf("%w: URL %q has no host", ErrRefused, raw)
	case u.User != nil:
		return fmt.Errorf("%w: URL %q carries user information", ErrRefused, raw)
	case scheme == "git" && u.Hostname() != loopback:
		return fmt.Errorf("%w: git:// URL %q is not on %s", ErrRefused, raw, loopback)
	}
	return nil
}

// isLoopbackDaemon reports whether raw, a URL CheckURL accepts, is one
// of the sandbox daemon.
func isLoopbackDaemon(raw string) bool {
	return strings.HasPrefix(raw, "git://")
}

// resolveURL returns the URL git fetches when told to fetch raw in
// dir: a url.<base>.insteadOf of the repository's config can rewrite
// it, and -c cannot remove that key, since it appends to a
// multi-valued one (05 5.1). git ls-remote --get-url applies the
// rewrite with git's own rules. A rewrite to a URL CheckURL refuses is
// refused, and so is one to the loopback daemon for a URL that was
// not; a rewrite to another HTTPS host is the stated limit of 05 5.1.
func (r *Runner) resolveURL(ctx context.Context, dir, raw string) (string, error) {
	if err := CheckURL(raw); err != nil {
		return "", err
	}
	out, err := r.output(ctx, dir, r.hostSettings(raw), "ls-remote", "--get-url", raw)
	if err != nil {
		return "", err
	}
	resolved := strings.TrimSuffix(out, "\n")
	if err := CheckURL(resolved); err != nil {
		return "", fmt.Errorf("%w: %q rewrites to it", err, raw)
	}
	if isLoopbackDaemon(resolved) && !isLoopbackDaemon(raw) {
		return "", fmt.Errorf("%w: %q rewrites to the loopback daemon %q", ErrRefused, raw, resolved)
	}
	return resolved, nil
}

// hostSettings returns the settings of the URL's host: the caFile of
// its gitHosts entry becomes http.<url>.sslCAInfo, which git applies to
// that host only (05 5.1).
func (r *Runner) hostSettings(raw string) []setting {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return nil
	}
	for _, h := range r.hosts {
		if h.CAFile != "" && strings.EqualFold(h.Host, u.Host) {
			return []setting{{"http.https://" + strings.ToLower(h.Host) + "/.sslCAInfo", h.CAFile}}
		}
	}
	return nil
}
