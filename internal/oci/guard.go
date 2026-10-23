// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"bytes"
	jsonv2 "encoding/json/v2"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// guard is the http.RoundTripper every request of one fetch goes
// through: go-containerregistry's ping, token exchange and manifest
// fetch all send their requests to it (remote.WithTransport), so it is
// where the guards of I28 that the library does not hold are held
// (REQ-0515). It refuses a URL that is not https and a redirect to one;
// counts the redirects; keeps the token realm on the registry's site;
// caps every body (the library reads manifests up to 100 MiB); verifies
// the sha256 of a manifest fetched by digest, and the registry's
// Docker-Content-Digest header (the library checks neither the header
// nor the shape of a token response); and checks the token response for
// the shape the product reads. One guard serves one fetch.
type guard struct {
	inner http.RoundTripper
	// host is the registry's name, the site its token realm must be on;
	// addr is its host:port as the library names it.
	host, addr string
	// testAddr is the Client's: the one host:port of a test that is let
	// through whatever its address.
	testAddr string

	mu sync.Mutex
	// realms are the token realms the registry's 401 named, which were
	// checked, as scheme://host/path.
	realms map[string]bool
}

// newGuard returns the guard of one fetch from the registry host (a name
// or a host:port).
func newGuard(inner http.RoundTripper, host string) *guard {
	return &guard{inner: inner, host: hostOf(host), addr: host, realms: map[string]bool{}}
}

// RoundTrip sends req if it passes the guards, and returns a response
// whose body has been read through its cap and checked.
func (g *guard) RoundTrip(req *http.Request) (*http.Response, error) {
	u := req.URL
	redirected := req.Response != nil
	switch {
	case redirected && u.Scheme != "https":
		return nil, fmt.Errorf("%w: to a URL that is not https", ErrRedirect)
	case redirected && u.User != nil:
		return nil, fmt.Errorf("%w: to a URL with credentials", ErrRedirect)
	case redirected && redirects(req) > maxRedirects:
		return nil, fmt.Errorf("%w: more than %d redirects", ErrRedirect, maxRedirects)
	case u.Scheme == "http" && u.Path == "/v2/":
		// The ping the library also sends over HTTP for a registry on a
		// local or private name.
		return nil, errHTTPFallback
	case u.Scheme != "https" || u.Host == "" || u.User != nil:
		return nil, fmt.Errorf("%w: %s", ErrInsecure, quote(u.Redacted()))
	}
	if err := g.checkLiteral(u); err != nil {
		return nil, err
	}
	resp, err := g.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	body, err := g.read(resp, req)
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return resp, nil
}

// checkLiteral refuses a request to an address literal that is not a
// public one, on every hop. The dial guard holds the same rule, but behind
// a proxy the dial is to the proxy, which would be asked to reach the
// address (CONNECT 10.1.2.3:443): the literal is judged here, before the
// proxy is. A name is left to the dial, or to the proxy.
func (g *guard) checkLiteral(u *url.URL) error {
	if _, err := netip.ParseAddr(u.Hostname()); err != nil || (g.testAddr != "" && u.Host == g.testAddr) {
		return nil
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	return checkPublic(net.JoinHostPort(u.Hostname(), port))
}

// redirects returns how many redirects led to req.
func redirects(req *http.Request) int {
	n := 0
	for r := req.Response; r != nil; r = r.Request.Response {
		n++
	}
	return n
}

// read reads the body of resp through the cap of its request, closes it,
// and applies the checks of the answer; it returns the body.
func (g *guard) read(resp *http.Response, req *http.Request) ([]byte, error) {
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized {
		if err := g.checkRealms(resp.Header.Values("Www-Authenticate")); err != nil {
			return nil, err
		}
	}
	digest, isManifest := manifestDigest(req.URL)
	limit := int64(MaxTokenSize)
	if isManifest {
		limit = MaxManifestSize
	}
	if resp.ContentLength > limit {
		return nil, fmt.Errorf("%w: the response says %d bytes, over the cap of %d", ErrTooLarge, resp.ContentLength, limit)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%w: the body passes the cap of %d", ErrTooLarge, limit)
	}
	if resp.StatusCode != http.StatusOK {
		return body, nil
	}
	switch {
	case isManifest:
		if err := verifyManifest(digest, body, resp.Header.Get("Docker-Content-Digest")); err != nil {
			return nil, err
		}
	case g.isRealm(req.URL):
		if err := checkToken(body); err != nil {
			return nil, err
		}
	}
	return body, nil
}

// manifestDigest returns the reference of a request for a manifest, and
// whether it is one.
func manifestDigest(u *url.URL) (string, bool) {
	_, ref, ok := strings.Cut(u.Path, "/manifests/")
	return ref, ok
}

// verifyManifest returns ErrDigestMismatch unless body is the content the
// digest names, and the registry's header, when it sends one, names it
// too (REQ-0515). A digest of another algorithm is refused: the product
// pins sha256 only (03 3.1, REQ-0189).
func verifyManifest(digest string, body []byte, header string) error {
	if !digestForm.MatchString(digest) {
		return fmt.Errorf("%w: %s is not sha256:<64 hex>", ErrDigestMismatch, quote(digest))
	}
	got, _, err := v1.SHA256(bytes.NewReader(body))
	if err != nil {
		return err
	}
	if got.String() != digest {
		return fmt.Errorf("%w: the content is %s, and %s was asked for", ErrDigestMismatch, got, digest)
	}
	if header != "" && header != digest {
		return fmt.Errorf("%w: the header Docker-Content-Digest is %s", ErrDigestMismatch, quote(header))
	}
	return nil
}

// checkRealms judges the challenges of a 401. A Bearer challenge names the
// realm, the token endpoint, and a hostile registry could name a service
// of the user's network: the realm must be https, with no user name, on the
// registry's site (ErrForeignRealm), and, if it is an address, a public one
// (ErrPrivateAddress); a challenge the guard cannot read exactly as the
// library does, a Bearer challenge without a realm or with two, and a 401
// that offers neither Bearer nor Basic, are of a shape the product does
// not read. It remembers the realms it let through, which are the URLs of
// the token requests (REQ-0515).
func (g *guard) checkRealms(headers []string) error {
	var known bool
	for _, h := range headers {
		c, ok := parseChallenge(h)
		if !ok {
			return shape("a WWW-Authenticate challenge of the 401 is not of a form the product reads")
		}
		switch c.scheme {
		case "basic":
			known = true
		case "bearer":
			known = true
			if err := g.checkRealm(c.params); err != nil {
				return err
			}
		}
	}
	if !known {
		return shape("the 401 offers neither a Bearer nor a Basic challenge")
	}
	return nil
}

// challenge is one challenge of a WWW-Authenticate value: its scheme in
// lower case, and its parameters in order, names in lower case.
type challenge struct {
	scheme string
	params []param
}

// param is one auth-param of a challenge.
type param struct{ name, value string }

// parseChallenge reads one WWW-Authenticate value as go-containerregistry
// v0.22.1 reads it (parseValueAndParams in
// pkg/v1/remote/internal/authchallenge), which is what decides the realm
// the library asks for a token: one challenge per value, a scheme token,
// then auth-params separated by commas, each a token, "=", and a token or
// a quoted-string in which a backslash escapes the next octet (RFC 9110
// 11.2 and 5.6.4, without the optional spaces around "="; 11.6.1 warns
// that two challenges on one line are not interoperable). The library
// stops at the first octet it cannot read, ignores the rest, and keeps
// the last value of a repeated parameter; ok is false when the value is
// not read to its end or holds an empty value, so the guard never judges
// a header the library reads otherwise.
func parseChallenge(h string) (c challenge, ok bool) {
	scheme, s := httpToken(h)
	if scheme == "" {
		return c, false
	}
	c.scheme = strings.ToLower(scheme)
	s = skipSpace(s)
	if s == "" {
		return c, true
	}
	for s = "," + s; s != ""; s = skipSpace(s) {
		if s[0] != ',' {
			return c, false
		}
		var name, value string
		name, s = httpToken(skipSpace(s[1:]))
		if name == "" || !strings.HasPrefix(s, "=") {
			return c, false
		}
		value, s, ok = tokenOrQuoted(s[1:])
		if !ok || value == "" {
			return c, false
		}
		c.params = append(c.params, param{name: strings.ToLower(name), value: value})
	}
	return c, true
}

// skipSpace drops the spaces, tabs and line breaks s starts with.
func skipSpace(s string) string { return strings.TrimLeft(s, " \t\r\n") }

// httpToken cuts the token s starts with (RFC 9110 5.6.2): ASCII that is
// neither a control nor a delimiter.
func httpToken(s string) (token, rest string) {
	i := 0
	for i < len(s) && s[i] > ' ' && s[i] < 0x7f && !strings.ContainsRune(`"(),/:;<=>?@[\]{}`, rune(s[i])) {
		i++
	}
	return s[:i], s[i:]
}

// tokenOrQuoted cuts the token or the quoted-string s starts with, a
// quoted-string unescaped; ok is false for a quoted-string without its
// closing quote.
func tokenOrQuoted(s string) (value, rest string, ok bool) {
	if !strings.HasPrefix(s, `"`) {
		value, rest = httpToken(s)
		return value, rest, true
	}
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '"':
			return b.String(), s[i+1:], true
		case '\\':
			i++
			if i == len(s) {
				return "", "", false
			}
		}
		b.WriteByte(s[i])
	}
	return "", "", false
}

// checkRealm judges the realm of one Bearer challenge. The library keeps
// the last realm of a challenge that names two, and RFC 9110 11.2 allows
// a name once per challenge, so a challenge that names more than one is
// refused, whichever would be judged.
func (g *guard) checkRealm(params []param) error {
	var found []string
	for _, p := range params {
		if p.name == "realm" {
			found = append(found, p.value)
		}
	}
	switch {
	case len(found) == 0:
		return shape("the Bearer challenge has no realm")
	case len(found) > 1:
		return shape("the Bearer challenge names more than one realm")
	}
	raw := found[0]
	realm, err := url.Parse(raw)
	if err != nil || realm.Host == "" {
		return shape("the realm of the challenge is not a URL")
	}
	if realm.Scheme != "https" || realm.User != nil {
		return fmt.Errorf("%w: the token realm %s", ErrInsecure, quote(raw))
	}
	if !sameSite(realm.Hostname(), g.host) {
		return fmt.Errorf("%w: %s", ErrForeignRealm, quote(realm.Hostname()))
	}
	// An address on the registry's site is the registry's own only if
	// it is the registry's host and port: another port of a loopback
	// address is another service.
	if _, err := netip.ParseAddr(realm.Hostname()); err == nil && realm.Host != g.addr {
		port := realm.Port()
		if port == "" {
			port = "443"
		}
		if err := checkPublic(net.JoinHostPort(realm.Hostname(), port)); err != nil {
			return err
		}
	}
	g.mu.Lock()
	g.realms[realmKey(realm)] = true
	g.mu.Unlock()
	return nil
}

// realmKey is the identity of a token endpoint: its URL without the
// query, which the library fills in.
func realmKey(u *url.URL) string { return "https://" + u.Host + u.Path }

// isRealm reports whether u is a token request to a realm checked.
func (g *guard) isRealm(u *url.URL) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.realms[realmKey(u)]
}

// checkToken returns a shape error unless body is a token response the
// product reads: JSON with typed members and a token or an access_token
// that holds no line break. The library reads the same two members and
// reports no token only as text.
func checkToken(body []byte) error {
	var t struct {
		Token       *string `json:"token"`
		AccessToken *string `json:"access_token"`
	}
	if err := jsonv2.Unmarshal(body, &t); err != nil {
		return shape("the token response does not decode as JSON with typed members")
	}
	var token string
	switch {
	case t.Token != nil && *t.Token != "":
		token = *t.Token
	case t.AccessToken != nil && *t.AccessToken != "":
		token = *t.AccessToken
	default:
		return shape("the token response has neither token nor access_token")
	}
	if strings.ContainsAny(token, "\r\n") {
		return shape("the token holds a line break")
	}
	return nil
}
