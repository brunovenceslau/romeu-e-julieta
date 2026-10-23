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
	"regexp"
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

// challengeStart finds where a challenge of a WWW-Authenticate value
// begins: a scheme followed by a parameter. A header may carry several
// challenges, and the library chooses the first it can handle, so each is
// judged.
var challengeStart = regexp.MustCompile(`(?i)(?:^|,)\s*([a-z][a-z0-9._~+-]*)\s+[a-z][a-z0-9_-]*\s*=`)

// realmParam finds the realm of a challenge, quoted or bare.
var realmParam = regexp.MustCompile(`(?i)\brealm\s*=\s*(?:"([^"]*)"|([^\s,"]+))`)

// checkRealms judges the challenges of a 401. A Bearer challenge names the
// realm, the token endpoint, and a hostile registry could name a service
// of the user's network: the realm must be https, with no user name, on the
// registry's site (ErrForeignRealm), and, if it is an address, a public one
// (ErrPrivateAddress); a Bearer challenge without a realm, and a 401 that
// offers neither Bearer nor Basic, are of a shape the product does not
// read. It remembers the realms it let through, which are the URLs of the
// token requests (REQ-0515).
func (g *guard) checkRealms(headers []string) error {
	var known bool
	for _, h := range headers {
		for _, c := range splitChallenges(h) {
			switch strings.ToLower(c.scheme) {
			case "basic":
				known = true
			case "bearer":
				known = true
				if err := g.checkRealm(c.params); err != nil {
					return err
				}
			}
		}
	}
	if !known {
		return shape("the 401 offers neither a Bearer nor a Basic challenge")
	}
	return nil
}

// challenge is one challenge of a WWW-Authenticate value.
type challenge struct{ scheme, params string }

// splitChallenges cuts a WWW-Authenticate value into its challenges.
func splitChallenges(h string) []challenge {
	starts := challengeStart.FindAllStringSubmatchIndex(h, -1)
	if len(starts) == 0 {
		// A scheme alone, such as "Basic" or "Negotiate".
		if f := strings.Fields(h); len(f) > 0 {
			return []challenge{{scheme: strings.TrimRight(f[0], ",")}}
		}
		return nil
	}
	out := make([]challenge, 0, len(starts))
	for i, m := range starts {
		end := len(h)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		out = append(out, challenge{scheme: h[m[2]:m[3]], params: h[m[3]:end]})
	}
	return out
}

// checkRealm judges the realm of one Bearer challenge.
func (g *guard) checkRealm(params string) error {
	found := realmParam.FindAllStringSubmatch(params, -1)
	if len(found) == 0 {
		return shape("the Bearer challenge has no realm")
	}
	for _, m := range found {
		raw := m[1] + m[2]
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
	}
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
