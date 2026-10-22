// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestDialTrustsTheRegistryAndAProxyOnly(t *testing.T) {
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

	conn, err := c.dial(trusted(t.Context(), addr), "tcp", addr)
	require.NoError(t, err, "the registry's own address")
	_ = conn.Close()

	_, err = c.dial(trusted(t.Context(), "127.0.0.1:1"), "tcp", addr)
	require.ErrorIs(t, err, ErrPrivateAddress, "trust is for one address")

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

func TestDialIsGivenTheTrustedAddressOfTheRegistry(t *testing.T) {
	f := newFakeRegistry(t)
	index, _ := publish(t, f)

	_, err := f.client().FetchWorkload(t.Context(), f.ref(index), amd64)

	require.NoError(t, err, "the registry on a loopback address is reached because the reference names it")
}

func TestRegistryAddress(t *testing.T) {
	assert.Equal(t, "ghcr.io:443", registryAddr("ghcr.io"))
	assert.Equal(t, "127.0.0.1:5000", registryAddr("127.0.0.1:5000"))
	assert.Equal(t, "registry-1.docker.io:443", registryAddr("registry-1.docker.io"))
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
	assert.Equal(t, int32(2), manifests.Load(), "one request, one after the token, and no third")
}
