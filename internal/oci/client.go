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
	// which only the registry the reference names, and a proxy the
	// environment names, may be.
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
}

// NewClient returns a Client with the guards on. The proxy of the
// environment is used, as for any HTTPS client, and the server's
// certificate is verified against the system roots.
func NewClient() *Client {
	c := &Client{}
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
	u, err := http.ProxyFromEnvironment(req)
	if u != nil && err == nil {
		port := u.Port()
		if port == "" {
			port = map[string]string{"http": "80", "https": "443", "socks5": "1080", "socks5h": "1080"}[u.Scheme]
		}
		c.proxies.Store(net.JoinHostPort(u.Hostname(), port), true)
	}
	return u, err
}

// trustKey is the context key of the address a request may reach even
// if it is not a public one.
type trustKey struct{}

// trusted returns ctx with addr, a host:port, as the one address its
// request may reach whatever it resolves to: the registry the reference
// names (a private registry on the user's network is the user's
// choice).
func trusted(ctx context.Context, addr string) context.Context {
	return context.WithValue(ctx, trustKey{}, addr)
}

// registryAddr returns the host:port a dial to the registry host has,
// 443 when it names no port.
func registryAddr(host string) string {
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	return net.JoinHostPort(host, "443")
}

// dial connects to addr and refuses an address that is not a public one,
// unless it is the registry's own or a proxy's. The check is on the
// address the name resolved to, at connect time, for each address tried,
// so a name that resolves to a loopback address (DNS rebinding) is
// refused the same as the address written out. Behind a proxy, the
// target's address is the proxy's to judge (REQ-0515).
func (c *Client) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	_, isProxy := c.proxies.Load(addr)
	d := &net.Dialer{Timeout: 10 * time.Second}
	if want, _ := ctx.Value(trustKey{}).(string); !isProxy && want != addr {
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
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() ||
		ip.IsMulticast() || reserved.Contains(ip) || ip == netip.AddrFrom4([4]byte{255, 255, 255, 255}) {
		return fmt.Errorf("%w: %s", ErrPrivateAddress, ip)
	}
	return nil
}

// reserved are the ranges of public-looking space that are not the
// internet's: carrier-grade NAT (RFC 6598) and the benchmarking range
// (RFC 2544), where a cloud's internal services can sit.
var reserved = prefixes{netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("198.18.0.0/15")}

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
