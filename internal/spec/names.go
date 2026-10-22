// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"errors"
	"fmt"
	"net/netip"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The value checks of the rules of rules.go that a pattern alone does
// not say. Each returns nil for a value it accepts, or an error whose
// text is the reason, which never quotes the value.

// branchName holds a branch name to the rules of git check-ref-format
// --branch, implemented in Go (03 3.1, REQ-0181): refs/heads/<name> is a
// valid ref name, and the name does not start with "-" and is not HEAD.
// It is stricter than git in two ways: the name is valid UTF-8, and "@",
// which --branch reads as the current branch, is refused.
func branchName(name string) error {
	switch {
	case name == "":
		return errors.New("is empty")
	case !utf8.ValidString(name):
		return errors.New("is not UTF-8")
	case strings.HasPrefix(name, "-"):
		return errors.New("starts with -")
	case name == "HEAD" || name == "@":
		return errors.New("is HEAD or @")
	case strings.Contains(name, ".."):
		return errors.New("holds two dots in a row")
	case strings.Contains(name, "@{"):
		return errors.New("holds two dots in a row")
	case strings.HasSuffix(name, "."):
		return errors.New("ends with a dot")
	case strings.ContainsFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f || strings.ContainsRune(" ~^:?*[\\", r) }):
		return errors.New("holds a character git refuses in a ref name")
	}
	for c := range strings.SplitSeq(name, "/") {
		switch {
		case c == "":
			return errors.New("has an empty component")
		case strings.HasPrefix(c, "."):
			return errors.New("has a component that starts with a dot")
		case strings.HasSuffix(c, ".lock"):
			return errors.New("has a component that ends with .lock")
		}
	}
	return nil
}

// ldhLabel is a lowercase DNS label: letters, digits and hyphens, a
// letter or digit at each end, at most 63 bytes.
var ldhLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// hostName holds a host name to 03 3.1 (REQ-0188): a lowercase host
// name of at least two labels, each an LDH label, so an IDNA name is
// written as its A-labels (xn--); no wildcard, port or IP literal,
// which the label form already refuses, and no label with hyphens in
// its third and fourth places but an A-label's, which IDNA reserves;
// and no label of 0x and hex digits, which some resolvers read as a
// number, so an address in disguise. The Punycode of an A-label is not
// decoded. It is the rule of a git host (gitHosts[].host and the host
// of a repo URL), which may be a self-hosted server under a private
// suffix, such as git.corp.internal.
func hostName(name string) error {
	if len(name) > 253 {
		return errors.New("is longer than 253 bytes")
	}
	if _, err := netip.ParseAddr(strings.Trim(name, "[]")); err == nil {
		return errors.New("is an IP literal")
	}
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return errors.New("has fewer than two labels")
	}
	for _, l := range labels {
		if !ldhLabel.MatchString(l) {
			return errors.New("has a label that is not lowercase letters, digits and hyphens (a wildcard, a port and an upper case letter are refused)")
		}
		if len(l) >= 4 && l[2:4] == "--" && !strings.HasPrefix(l, "xn--") {
			return errors.New("has a label with -- in its third and fourth places that is no A-label")
		}
	}
	if !strings.ContainsFunc(labels[len(labels)-1], func(r rune) bool { return r < '0' || r > '9' }) {
		return errors.New("ends with a label of digits only")
	}
	if slices.ContainsFunc(labels, isHexLabel) {
		return errors.New("has a label of 0x and hex digits, which a resolver may read as an address")
	}
	return nil
}

// domainName holds an egress domain to 03 3.1 (REQ-0188): a hostName
// that is not under a special-use name, which a resolver may answer
// from the host or its local network, so an egress entry could reach
// it (see specialUse).
func domainName(name string) error {
	if err := hostName(name); err != nil {
		return err
	}
	labels := strings.Split(name, ".")
	if slices.Contains(specialUse, labels[len(labels)-1]) || name == "home.arpa" || strings.HasSuffix(name, ".home.arpa") {
		return errors.New("ends with a special-use name a resolver may map to the host or its local network (" + specialUseText + ")")
	}
	return nil
}

// specialUse are the last labels domainName refuses, besides the suffix
// home.arpa: localhost (RFC 6761, 6.3), test and invalid (RFC 6761, 6.1
// and 6.4), local (multicast DNS, RFC 6762), home.arpa (RFC 8375),
// onion (RFC 7686), internal (reserved for private use by ICANN, 2024),
// and the names that home and corporate networks use without a
// registration (localdomain, lan, home, corp, intranet, private; RFC
// 6762, appendix G). example (RFC 2606) is not here: it is
// documentation, and it cannot be registered or resolved.
var specialUse = []string{"localhost", "local", "internal", "localdomain", "lan", "home", "corp", "intranet", "private", "test", "invalid", "onion"}

// specialUseText lists specialUse and home.arpa for a message.
var specialUseText = strings.Join(specialUse, ", ") + ", home.arpa"

// isHexLabel reports whether l is 0x followed by at least one hex digit
// and nothing else.
func isHexLabel(l string) bool {
	rest, ok := strings.CutPrefix(l, "0x")
	return ok && rest != "" && strings.Trim(rest, "0123456789abcdef") == ""
}

// urlSegment is an owner or a repository name of a repo URL.
var urlSegment = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)

// repoURL holds a repo URL to 03 3.1 (REQ-0177): https://<host>/<owner>/
// <repo>, with an optional .git; one owner segment, so no GitLab
// subgroups, no port, user, query or fragment. The host may be in any
// case; it is a host name once lowercased.
func repoURL(raw string) error {
	rest, ok := strings.CutPrefix(raw, "https://")
	if !ok {
		return errors.New("does not start with https://")
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 3 {
		return errors.New("is not https://<host>/<owner>/<repo>")
	}
	if err := hostName(strings.ToLower(parts[0])); err != nil {
		return errors.New("has a host that " + err.Error())
	}
	repo := strings.TrimSuffix(parts[2], ".git")
	for _, s := range []string{parts[1], repo} {
		if !urlSegment.MatchString(s) || s == "." || s == ".." {
			return errors.New("has an owner or repository that is not letters, digits, _, . and -")
		}
	}
	return nil
}

// NormalizeURL returns the form two repo URLs are compared in (03 3.1,
// REQ-0177): lowercased, with a trailing .git removed.
func NormalizeURL(raw string) string {
	return strings.TrimSuffix(strings.ToLower(raw), ".git")
}

// hasControl reports whether s holds a control character.
func hasControl(s string) bool {
	return strings.ContainsFunc(s, unicode.IsControl)
}

// absolutePath holds a path that must be absolute: it starts with /, is
// clean (path.Clean leaves it as it is) and holds no control character.
func absolutePath(p string) error {
	switch {
	case !strings.HasPrefix(p, "/"):
		return errors.New("is not an absolute path")
	case path.Clean(p) != p:
		return errors.New("is not a clean path (no . or .. element, no doubled or trailing /)")
	case hasControl(p):
		return errors.New("holds a control character")
	}
	return nil
}

// relativePath holds a relative path inside a repository (03 3.1,
// REQ-0187): clean, with no leading / and no .. element.
func relativePath(p string) error {
	switch {
	case strings.HasPrefix(p, "/"):
		return errors.New("starts with /")
	case path.Clean(p) != p:
		return errors.New("is not a clean path (no . element but the path ., no doubled or trailing /)")
	case p == ".." || strings.HasPrefix(p, "../"):
		return errors.New("has a .. element")
	case hasControl(p):
		return errors.New("holds a control character")
	}
	return nil
}

// printableArg holds a kit argument's value (03 3.1, REQ-0183):
// printable UTF-8, so no newline or other control character.
func printableArg(v string) error {
	if !utf8.ValidString(v) {
		return errors.New("is not UTF-8")
	}
	if strings.ContainsFunc(v, func(r rune) bool { return !unicode.IsPrint(r) }) {
		return errors.New("holds a character that is not printable, such as a newline")
	}
	return nil
}

// helperName is the name of a git-credential-<name> helper.
var helperName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// credentialHelper holds tools.gitCredentialHelper (03 3.4): the name of
// a git-credential-<name> helper, or an absolute path.
func credentialHelper(v string) error {
	if helperName.MatchString(v) {
		return nil
	}
	if err := absolutePath(v); err != nil {
		return errors.New("is neither a helper name of lowercase letters, digits and hyphens nor an absolute path")
	}
	return nil
}

// portSpec holds a port spec past its pattern: each port is from 1 to
// 65535. The pattern admits a loopback host IP only, which is cut off.
func portSpec(v string) error {
	spec, _, _ := strings.Cut(v, "/")
	spec = strings.TrimPrefix(spec, "127.0.0.1:")
	for p := range strings.SplitSeq(spec, ":") {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 || p[0] == '0' {
			return errors.New("has a port outside 1 to 65535")
		}
	}
	return nil
}

// memoryLimit holds a memory limit past its pattern: at least 512 MiB,
// sbx's minimum.
func memoryLimit(v string) error {
	n, err := strconv.Atoi(v[:len(v)-1])
	if err != nil || (v[len(v)-1] == 'm' && n < 512) {
		return errors.New("is below 512m")
	}
	return nil
}

// excludePath holds an entry of salvage.excludeIgnored: a relative path
// as relativePath holds one, with an optional trailing /, that does not
// start with -.
func excludePath(v string) error {
	p := strings.TrimSuffix(v, "/")
	switch {
	case p == "" || p == ".":
		return errors.New("is empty")
	case strings.HasPrefix(p, "-"):
		return errors.New("starts with -")
	}
	return relativePath(p)
}

// noControl holds free text: no control character, so no newline, NUL
// or escape sequence.
func noControl(v string) error {
	if !utf8.ValidString(v) {
		return errors.New("is not UTF-8")
	}
	if hasControl(v) {
		return errors.New("holds a control character")
	}
	return nil
}

// ValidDomain returns why name is not an egress domain or host name by
// the rule of 03 3.1 (REQ-0188), or nil: the one check of the domain
// rule, for every source of 07 7.5 (the spec, the catalog, the
// lock-derived and the kit-declared domains).
func ValidDomain(name string) error {
	if err := domainName(name); err != nil {
		return fmt.Errorf("the domain %w", err)
	}
	return nil
}

// Workload is a workload reference, <registry>/<repository>@sha256:<64
// hex> (03 3.1, REQ-0189).
type Workload struct {
	// Repository is <registry>/<repository>, the form of an entry of
	// workloadRepositories.
	Repository string
	// Digest is "sha256:" and 64 lowercase hex digits.
	Digest string
}

// workloadForm is the pattern of a workload reference.
var workloadForm = regexp.MustCompile(workloadPattern)

// ParseWorkload reads a workload reference by the rule of 03 3.1, the one
// check of its form for the spec and internal/oci.
func ParseWorkload(ref string) (Workload, error) {
	if !workloadForm.MatchString(ref) {
		return Workload{}, errors.New("the workload is not <registry>/<repository>@sha256:<64 hex>")
	}
	repo, digest, _ := strings.Cut(ref, "@")
	return Workload{Repository: repo, Digest: digest}, nil
}
