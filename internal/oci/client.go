// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
)

// The caps of the client (REQ-0515): every response is read through a
// limit, so a registry cannot make the client hold more than this.
const (
	// MaxManifestSize is the most bytes of an index or a manifest: the 4
	// MiB the distribution spec says a registry supports (a manifest
	// push of at least that size), which no real image comes near.
	// go-containerregistry reads up to 100 MiB, so the guard holds this
	// cap itself.
	MaxManifestSize = 4 << 20
	// MaxTokenSize is the most bytes of a token response.
	MaxTokenSize = 64 << 10
	// maxRedirects is the most redirects a request follows.
	maxRedirects = 5
	// requestTimeout bounds a whole fetch, every hop and body read
	// included.
	requestTimeout = 60 * time.Second
)

// The refusals of the guards of the client. They are wrapped with the
// registry's name, and FetchWorkload gives them the row upstream-shape.
var (
	// ErrInsecure is a URL that is not https, or that holds credentials.
	ErrInsecure = errors.New("oci: not an https URL")
	// ErrRedirect is a redirect to a URL that is not https, or one too
	// many.
	ErrRedirect = errors.New("oci: redirect refused")
	// ErrTooLarge is a response over its cap.
	ErrTooLarge = errors.New("oci: response over its cap")
	// ErrDigestMismatch is content whose sha256 is not the digest it was
	// asked for by.
	ErrDigestMismatch = errors.New("oci: content does not match its digest")
	// ErrForeignRealm is a token realm outside the site of the registry.
	ErrForeignRealm = errors.New("oci: token realm outside the registry's site")
	// ErrPrivateAddress is a connection to an address that is not a
	// public one (a loopback, private, link-local or reserved address),
	// which only a proxy the environment names may be: a registry on a
	// private network is not read in v1, whatever the reference names
	// (RJ-322).
	ErrPrivateAddress = errors.New("oci: address is not a public one")
	// ErrCredentials is a registry that asks for credentials.
	ErrCredentials = errors.New("oci: the registry asks for credentials")
	// errHTTPFallback is the plain-HTTP ping go-containerregistry tries
	// beside the HTTPS one for a registry on a local or private name:
	// refused, and not a guard's refusal of the registry's answer.
	errHTTPFallback = errors.New("oci: plain HTTP is never used")
)

// Client is the guarded client of the registries (REQ-0515, I28). It
// reads anonymously: it holds no credential, reads no Docker config and
// runs no credential helper, whatever DOCKER_CONFIG or the home
// directory hold (REQ-0210); it speaks HTTPS only and follows only HTTPS
// redirects; it verifies the digest of every hop; and it caps every
// response. The registry protocol (the ping, the token exchange, the
// fetch of a manifest by digest) is go-containerregistry's; the guards
// are the transport it is given (NewClient) and the guard that wraps it
// for one fetch (guard.go). A Client is safe for concurrent use.
type Client struct {
	// transport is the one connection pool and the dial guard.
	transport *http.Transport
	// proxies are the host:port of the proxies the environment named for
	// a request: a connection to one is a connection the user chose.
	proxies sync.Map
	// proxyOf names the proxy of a request: http.ProxyFromEnvironment,
	// which reads the environment once per process, so a test sets its
	// own.
	proxyOf func(*http.Request) (*url.URL, error)
	// testAddr is the one host:port a dial and the guard let through
	// whatever its address: the fake registry of a test, on the
	// loopback. A Client of NewClient leaves it empty.
	testAddr string
}

// NewClient returns a Client with the guards on. The proxy of the
// environment is used, as for any HTTPS client, and the server's
// certificate is verified against the system roots.
func NewClient() *Client {
	c := &Client{proxyOf: http.ProxyFromEnvironment}
	c.transport = &http.Transport{
		Proxy:                  c.proxy,
		DialContext:            c.dial,
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:    10 * time.Second,
		ResponseHeaderTimeout:  30 * time.Second,
		MaxResponseHeaderBytes: maxHeaderBytes,
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           4,
		IdleConnTimeout:        30 * time.Second,
	}
	return c
}

// maxHeaderBytes caps the headers of a response, which no other cap
// reaches.
const maxHeaderBytes = 64 << 10

// proxy is the proxy of the environment, remembered so that dial can tell
// a connection to it from one to a target.
func (c *Client) proxy(req *http.Request) (*url.URL, error) {
	u, err := c.proxyOf(req)
	if u != nil && err == nil {
		port := u.Port()
		if port == "" {
			port = map[string]string{"http": "80", "https": "443", "socks5": "1080", "socks5h": "1080"}[u.Scheme]
		}
		c.proxies.Store(net.JoinHostPort(u.Hostname(), port), true)
	}
	return u, err
}

// dial connects to addr and refuses an address that is not a public one,
// unless it is a proxy's. The registry the reference names is no
// exception: its name may resolve to anything. The check is on the
// address the name resolved to, at connect time, for each address tried,
// so a name that resolves to a loopback address (DNS rebinding) is
// refused the same as the address written out. Behind a proxy, the
// address a target name resolves to is the proxy's to judge, and an
// address literal is the guard's (REQ-0515).
func (c *Client) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	_, isProxy := c.proxies.Load(addr)
	d := &net.Dialer{Timeout: 10 * time.Second}
	if !isProxy && (c.testAddr == "" || addr != c.testAddr) {
		d.Control = func(_, address string, _ syscall.RawConn) error { return checkPublic(address) }
	}
	return d.DialContext(ctx, network, addr)
}

// checkPublic returns ErrPrivateAddress unless address, an ip:port, is a
// public unicast address.
func checkPublic(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrPrivateAddress, quote(address))
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrPrivateAddress, quote(address))
	}
	if !public(ip) {
		return fmt.Errorf("%w: %s", ErrPrivateAddress, ip.Unmap())
	}
	return nil
}

// public reports whether ip is a public unicast address. An IPv6 address
// that carries an IPv4 one for the network to translate (NAT64, 6to4) is
// as public as the IPv4 address it carries.
func public(ip netip.Addr) bool {
	ip = ip.Unmap()
	if ip.Is6() {
		b := ip.As16()
		switch {
		case nat64.Contains(ip):
			return public(netip.AddrFrom4([4]byte(b[12:16])))
		case sixToFour.Contains(ip):
			return public(netip.AddrFrom4([4]byte(b[2:6])))
		}
	}
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified() &&
		!ip.IsMulticast() && !reserved.Contains(ip) && ip != netip.AddrFrom4([4]byte{255, 255, 255, 255})
}

// The translation prefixes whose addresses carry an IPv4 address: the
// well-known NAT64 prefix (RFC 6052), the last four bytes, and 6to4 (RFC
// 3056), the two bytes after the prefix.
var (
	nat64     = netip.MustParsePrefix("64:ff9b::/96")
	sixToFour = netip.MustParsePrefix("2002::/16")
)

// reserved are the ranges of public-looking space that are not the
// internet's (RFC 6890 and its updates): "this network" (0.0.0.0/8),
// carrier-grade NAT (RFC 6598), the IETF protocol assignments, the
// documentation ranges of IPv4 and IPv6 (RFC 5737, RFC 3849, RFC 9637),
// the benchmarking range (RFC 2544), the former class E (240.0.0.0/4),
// and the local-use NAT64 prefix (RFC 8215), whose translation is the
// network's own. A cloud's internal services can sit in any of them.
var reserved = prefixes{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("3fff::/20"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
}

// prefixes is a list of address ranges.
type prefixes []netip.Prefix

// Contains reports whether any range holds ip.
func (ps prefixes) Contains(ip netip.Addr) bool {
	return slices.ContainsFunc(ps, func(p netip.Prefix) bool { return p.Contains(ip) })
}

// sameSite reports whether two host names belong to one site: they are
// equal, or they share their registrable domain, so a registry may name a
// token service of its own (registry-1.docker.io and auth.docker.io are
// docker.io). It is a heuristic without the public suffix list, which
// would be a new dependency: the last two labels, or three where the
// second to last is co, com, org, net, gov, edu or ac under a two-letter
// top level (example.co.uk). An address literal is its own site.
func sameSite(a, b string) bool {
	a, b = strings.TrimSuffix(strings.ToLower(a), "."), strings.TrimSuffix(strings.ToLower(b), ".")
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	if _, err := netip.ParseAddr(a); err == nil {
		return false
	}
	if _, err := netip.ParseAddr(b); err == nil {
		return false
	}
	sa, sb := site(a), site(b)
	return sa != "" && sa == sb
}

// site returns the registrable domain of a name, or "" when it has
// fewer than two labels.
func site(name string) string {
	labels := strings.Split(name, ".")
	n := 2
	if len(labels) >= 3 && len(labels[len(labels)-1]) == 2 &&
		slices.Contains([]string{"co", "com", "org", "net", "gov", "edu", "ac"}, labels[len(labels)-2]) {
		n = 3
	}
	if len(labels) < n {
		return ""
	}
	return strings.Join(labels[len(labels)-n:], ".")
}
