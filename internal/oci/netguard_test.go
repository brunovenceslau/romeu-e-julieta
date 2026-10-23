// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSameSite(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"registry-1.docker.io", "auth.docker.io", true},
		{"docker.io", "auth.docker.io", true},
		{"ghcr.io", "ghcr.io", true},
		{"ghcr.io", "GHCR.io.", true},
		{"quay.io", "auth.quay.io", true},
		{"registry.example.co.uk", "auth.example.co.uk", true},
		{"registry.example.co.uk", "auth.other.co.uk", false},
		{"registry.example.com", "auth.evil.com", false},
		{"registry.example.com", "example.com.evil.com", false},
		{"registry.example.com", "evilexample.com", false},
		{"ghcr.io", "169.254.169.254", false},
		{"127.0.0.1", "127.0.0.1", true},
		{"127.0.0.1", "127.0.0.2", false},
		{"localhost", "localhost", true},
		{"localhost", "evil", false},
		{"a.io", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.a+"/"+tt.b, func(t *testing.T) {
			assert.Equal(t, tt.want, sameSite(tt.a, tt.b))
		})
	}
}

func TestCheckPublic(t *testing.T) {
	for _, addr := range []string{
		"127.0.0.1:443", "[::1]:443", "10.1.2.3:443", "172.16.0.9:443", "192.168.1.1:443",
		"169.254.169.254:80", "[fe80::1]:443", "[fd00::1]:443", "100.64.0.1:443", "0.0.0.0:443",
		"[::]:443", "[::ffff:127.0.0.1]:443", "[::ffff:10.0.0.1]:443", "224.0.0.1:443", "[ff02::1]:443",
		"198.18.0.1:443", "255.255.255.255:443",
	} {
		t.Run(addr, func(t *testing.T) {
			require.ErrorIs(t, checkPublic(addr), ErrPrivateAddress)
		})
	}
	for _, addr := range []string{"8.8.8.8:443", "[2606:4700:4700::1111]:443", "93.184.216.34:443"} {
		t.Run(addr, func(t *testing.T) {
			require.NoError(t, checkPublic(addr))
		})
	}
	require.ErrorIs(t, checkPublic("not an address"), ErrPrivateAddress, "an address that is not an IP is refused")
}

func TestGuardRefusesATokenRealmOutsideTheSite(t *testing.T) {
	f := newFakeRegistry(t)
	f.override = func(w http.ResponseWriter, _ *http.Request) bool {
		w.Header().Set("Www-Authenticate", `Bearer realm="https://169.254.169.254/latest/meta-data",service="x"`)
		w.WriteHeader(http.StatusUnauthorized)
		return true
	}

	_, err := f.client().FetchWorkload(t.Context(), f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

	require.ErrorIs(t, err, ErrForeignRealm)
	_ = shapeOf(t, err, f)
}

// loopbackWitness is a second TLS server on the loopback, another port
// than the fake registry's, that counts the requests it gets.
func loopbackWitness(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"token":"x"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestGuardRefusesATokenRealmOnAnotherLoopbackPort(t *testing.T) {
	witness, hits := loopbackWitness(t)
	f := newFakeRegistry(t)
	f.override = func(w http.ResponseWriter, _ *http.Request) bool {
		// The same host as the registry, so the site check passes: the
		// address check is what refuses it.
		w.Header().Set("Www-Authenticate", `Bearer realm="`+witness.URL+`/token"`)
		w.WriteHeader(http.StatusUnauthorized)
		return true
	}
	c := f.client()
	trust(t, c, witness)

	_, err := c.FetchWorkload(t.Context(), f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

	require.ErrorIs(t, err, ErrPrivateAddress)
	_ = shapeOf(t, err, f)
	assert.Zero(t, hits.Load(), "the witness was never asked")
}

func TestGuardRefusesARedirectToAPrivateAddress(t *testing.T) {
	for name, host := range map[string]func(port string) string{
		"an address literal":         func(p string) string { return "127.0.0.1:" + p },
		"a name that resolves to it": func(p string) string { return "localhost:" + p }, // stands for a DNS rebinding
	} {
		t.Run(name, func(t *testing.T) {
			witness, hits := loopbackWitness(t)
			_, port, _ := net.SplitHostPort(strings.TrimPrefix(witness.URL, "https://"))
			f := newFakeRegistry(t)
			f.override = func(w http.ResponseWriter, r *http.Request) bool {
				http.Redirect(w, r, "https://"+host(port)+"/v2/", http.StatusFound)
				return true
			}
			c := f.client()
			trust(t, c, witness)

			_, err := c.FetchWorkload(t.Context(), f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

			require.ErrorIs(t, err, ErrPrivateAddress)
			assert.Zero(t, hits.Load())
		})
	}
}

func TestDialTrustsATestAddressAndAProxyOnly(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	addr := ln.Addr().String()
	c := NewClient()

	_, err = c.dial(t.Context(), "tcp", addr)
	require.ErrorIs(t, err, ErrPrivateAddress, "an untrusted loopback address")

	c.testAddr = addr
	conn, err := c.dial(t.Context(), "tcp", addr)
	require.NoError(t, err, "the address a test sets")
	_ = conn.Close()

	c.testAddr = "127.0.0.1:1"
	_, err = c.dial(t.Context(), "tcp", addr)
	require.ErrorIs(t, err, ErrPrivateAddress, "the test address is one address")

	c.testAddr = ""
	c.proxies.Store(addr, true)
	conn, err = c.dial(t.Context(), "tcp", addr)
	require.NoError(t, err, "a proxy the environment names")
	_ = conn.Close()
}

func TestTransportCapsTheResponseHeaders(t *testing.T) {
	f := newFakeRegistry(t)
	f.override = func(w http.ResponseWriter, _ *http.Request) bool {
		w.Header().Set("X-Padding", strings.Repeat("a", 128<<10))
		return false
	}

	_, err := f.client().FetchWorkload(t.Context(), f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeded")
}

func TestDialReachesTheFakeRegistryOnlyThroughTheTestAddress(t *testing.T) {
	f := newFakeRegistry(t)
	index, _ := publish(t, f)

	_, err := f.client().FetchWorkload(t.Context(), f.ref(index), amd64)

	require.NoError(t, err, "the fake registry on the loopback is reached because the test sets its address")
}

func TestGuardRefusesARedirectWithAUserName(t *testing.T) {
	f := newFakeRegistry(t)
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		http.Redirect(w, r, "https://user@"+r.Host+r.URL.Path, http.StatusFound)
		return true
	}

	_, err := f.client().FetchWorkload(t.Context(), f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

	require.ErrorIs(t, err, ErrRedirect)
}

func TestFetchWorkloadStopsAfterASecondUnauthorized(t *testing.T) {
	f := newFakeRegistry(t)
	var manifests atomic.Int32
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/token" {
			_, _ = w.Write([]byte(`{"token":"t"}`))
			return true
		}
		manifests.Add(1)
		w.Header().Set("Www-Authenticate", `Bearer realm="https://`+r.Host+`/token"`)
		w.WriteHeader(http.StatusUnauthorized)
		return true
	}

	_, err := f.client().FetchWorkload(t.Context(), f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

	require.ErrorIs(t, err, ErrCredentials)
	assert.Equal(t, int32(3), manifests.Load(), "the ping, the manifest, the one after the token, and no more")
}

// The challenges of a 401, judged as go-containerregistry v0.22.1 reads
// them (pkg/v1/remote/internal/authchallenge): one challenge per header
// value, quoted strings with backslash escapes, and the last value of a
// repeated parameter wins.
func TestGuardJudgesTheChallengeTheLibraryReads(t *testing.T) {
	const reg = "registry.example.com"
	refused := []struct {
		name, header string
	}{
		{
			// A quoted comma hides a second realm from a reader that
			// splits on commas; the library reads one Bearer challenge
			// whose last realm, the evil one, wins.
			name:   "a second realm after a quoted comma",
			header: `Bearer realm="https://` + reg + `/token", service="a, Basic z=1", realm="https://evil.example.invalid/token"`,
		},
		{
			name:   "two realms",
			header: `Bearer realm="https://` + reg + `/token", realm="https://evil.example.invalid/token"`,
		},
		{
			name:   "a second realm after an escaped quote",
			header: `Bearer realm="https://` + reg + `/token", service="a\", Basic z=1", realm="https://evil.example.invalid/token"`,
		},
		{name: "text left over", header: `Bearer realm="https://` + reg + `/token" junk`},
		{name: "a bare realm the library cuts at its colon", header: `Bearer realm=https://` + reg + `/token`},
		{name: "an empty realm", header: `Bearer realm="", service="x"`},
		// The library stops at an empty value and reads no realm after it.
		{name: "an empty value before the realm", header: `Bearer service="", realm="https://` + reg + `/token"`},
		{name: "an unterminated quote", header: `Bearer realm="https://` + reg + `/token`},
		{name: "a trailing comma", header: `Bearer realm="https://` + reg + `/token",`},
		{name: "a second challenge in one value", header: `Basic realm="a", Bearer realm="https://evil.example.invalid/token"`},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			g := newGuard(nil, reg)

			err := g.checkRealms([]string{tt.header})

			require.Error(t, err)
			_, isShape := errors.AsType[*shapeError](err)
			assert.True(t, isShape || errors.Is(err, ErrForeignRealm), "a shape or a foreign realm: %v", err)
			assert.False(t, g.isRealm(mustURL(t, "https://evil.example.invalid/token")), "the evil realm is never remembered as a token endpoint")
		})
	}
	accepted := []struct {
		name, header, realm string
	}{
		{name: "the form of a real registry", header: `Bearer realm="https://` + reg + `/token",service="` + reg + `",scope="repository:org/kit:pull"`, realm: "https://" + reg + "/token"},
		{
			// The escaped quotes keep the text after them inside the
			// value of service: the one realm is the registry's.
			name:   "a realm-like text inside an escaped value",
			header: `Bearer realm="https://` + reg + `/token", service="x\", realm=\"https://evil.example.invalid/token"`,
			realm:  "https://" + reg + "/token",
		},
		{name: "an escape inside the realm", header: `Bearer realm="https://` + reg + `/to\ken"`, realm: "https://" + reg + "/token"},
		{name: "a scheme alone", header: `Basic`},
		{name: "a Basic challenge", header: `Basic realm="registry"`},
	}
	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			g := newGuard(nil, reg)

			require.NoError(t, g.checkRealms([]string{tt.header}))

			if tt.realm != "" {
				assert.True(t, g.isRealm(mustURL(t, tt.realm)))
			}
		})
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

// connectProxy is an HTTP proxy on the loopback that tunnels a CONNECT
// to the one address it may reach, refuses any other, and records the
// targets it was asked for.
type connectProxy struct {
	srv     *httptest.Server
	mu      sync.Mutex
	targets []string
}

func newConnectProxy(t *testing.T, reach string) *connectProxy {
	t.Helper()
	p := &connectProxy{}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.targets = append(p.targets, r.Host)
		p.mu.Unlock()
		if r.Method != http.MethodConnect || r.Host != reach {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		up, err := net.Dial("tcp", reach)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
		conn, buf, err := http.NewResponseController(w).Hijack()
		if err != nil {
			_ = up.Close()
			return
		}
		go func() { _, _ = io.Copy(up, buf); _ = up.Close() }()
		_, _ = io.Copy(conn, up)
		_ = conn.Close()
	}))
	t.Cleanup(p.srv.Close)
	return p
}

// asked returns the targets the proxy was asked for.
func (p *connectProxy) asked() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.targets...)
}

// use makes c send its requests through the proxy.
func (p *connectProxy) use(c *Client) {
	u, _ := url.Parse(p.srv.URL)
	c.proxyOf = func(*http.Request) (*url.URL, error) { return u, nil }
}

func TestGuardRefusesARedirectToAPrivateAddressBehindAProxy(t *testing.T) {
	for _, target := range []string{"10.1.2.3", "169.254.169.254", "[fd00::1]", "127.0.0.1:9"} {
		t.Run(target, func(t *testing.T) {
			f := newFakeRegistry(t)
			registry := strings.TrimPrefix(f.srv.URL, "https://")
			proxy := newConnectProxy(t, registry)
			f.override = func(w http.ResponseWriter, r *http.Request) bool {
				http.Redirect(w, r, "https://"+target+"/v2/", http.StatusFound)
				return true
			}
			c := f.client()
			proxy.use(c)

			_, err := c.FetchWorkload(t.Context(), f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

			require.ErrorIs(t, err, ErrPrivateAddress)
			assert.Equal(t, []string{registry}, proxy.asked(), "the proxy is asked for the registry only")
		})
	}
	t.Run("a public address is the proxy's to reach", func(t *testing.T) {
		f := newFakeRegistry(t)
		registry := strings.TrimPrefix(f.srv.URL, "https://")
		proxy := newConnectProxy(t, registry)
		f.override = func(w http.ResponseWriter, r *http.Request) bool {
			http.Redirect(w, r, "https://8.8.8.8/v2/", http.StatusFound)
			return true
		}
		c := f.client()
		proxy.use(c)

		_, err := c.FetchWorkload(t.Context(), f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

		require.Error(t, err, "the fake proxy refuses it")
		assert.Equal(t, []string{registry, "8.8.8.8:443"}, proxy.asked())
	})
}

func TestProxyIsRememberedForTheDial(t *testing.T) {
	for raw, want := range map[string]string{
		"http://proxy.example":      "proxy.example:80",
		"https://proxy.example":     "proxy.example:443",
		"http://proxy.example:3128": "proxy.example:3128",
		"socks5://[fd00::1]":        "[fd00::1]:1080",
	} {
		t.Run(raw, func(t *testing.T) {
			c := NewClient()
			u := mustURL(t, raw)
			c.proxyOf = func(*http.Request) (*url.URL, error) { return u, nil }
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://registry.example/v2/", nil)
			require.NoError(t, err)

			got, err := c.proxy(req)

			require.NoError(t, err)
			assert.Equal(t, u, got)
			_, ok := c.proxies.Load(want)
			assert.True(t, ok, "%s is remembered", want)
		})
	}
	t.Run("no proxy", func(t *testing.T) {
		c := NewClient()
		c.proxyOf = func(*http.Request) (*url.URL, error) { return nil, nil }
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://registry.example/v2/", nil)
		require.NoError(t, err)

		got, err := c.proxy(req)

		require.NoError(t, err)
		assert.Nil(t, got)
		n := 0
		c.proxies.Range(func(any, any) bool { n++; return true })
		assert.Zero(t, n)
	})
}

func TestARegistryOnAPrivateAddressIsRefused(t *testing.T) {
	f := newFakeRegistry(t)
	index, _ := publish(t, f)
	registry := strings.TrimPrefix(f.srv.URL, "https://")
	_, port, err := net.SplitHostPort(registry)
	require.NoError(t, err)
	t.Run("an address literal the reference names", func(t *testing.T) {
		c := NewClient()
		trust(t, c, f.srv)
		before := len(f.requests())

		_, err := c.FetchWorkload(t.Context(), registry+"/org/kit@"+index, amd64)

		require.ErrorIs(t, err, ErrPrivateAddress)
		assert.Len(t, f.requests(), before, "the registry was never reached")
	})
	t.Run("an address literal behind a proxy", func(t *testing.T) {
		proxy := newConnectProxy(t, registry)
		c := NewClient()
		proxy.use(c)

		_, err := c.FetchWorkload(t.Context(), "169.254.169.254/org/kit@"+index, amd64)

		require.ErrorIs(t, err, ErrPrivateAddress)
		assert.Empty(t, proxy.asked(), "the proxy is never asked for it")
	})
	t.Run("a name that resolves to it", func(t *testing.T) {
		// The reference form wants a dotted name, which a test cannot
		// resolve without the network: the dial is asked directly.
		_, err := NewClient().dial(t.Context(), "tcp", "localhost:"+port)

		require.ErrorIs(t, err, ErrPrivateAddress)
	})
}
