// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package pin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redirectHarness is a mise run whose checksum file answers with a
// redirect to location, built from the harness's own URL.
func redirectHarness(t *testing.T, location func(h *harness) string) *harness {
	t.Helper()
	routes := miseRelease(t, miseSums, miseSig)
	h := newHarness(t, routes)
	routes["/mise/v2026.10.3/SHASUMS256.txt"] = route{status: http.StatusFound, headers: map[string]string{"Location": location(h)}}
	h.repo.Write("kits/mise-minisign.pub", string(fixture(t, misePub)))
	return h
}

// TestFetchRefusesARedirect: a redirect to http, or to a host the
// source does not name, is refused, and nothing is written.
func TestFetchRefusesARedirect(t *testing.T) {
	cases := []struct {
		name     string
		location func(h *harness) string
		want     string
	}{
		{"to_http", func(h *harness) string {
			return "http://" + strings.TrimPrefix(h.url, "https://") + "/mise/v2026.10.3/SHASUMS256.txt.minisig"
		}, "redirect refused: http://"},
		{"to_a_foreign_host", func(*harness) string { return "https://foreign.example/SHASUMS256.txt" }, "redirect refused: https://foreign.example/SHASUMS256.txt is not on a host of this source"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := redirectHarness(t, c.location)
			assert.Equal(t, exitError, h.run(t, "mise", "2026.10.3"))
			assert.Contains(t, h.stderr.String(), c.want)
			assert.False(t, h.exists("kits/pins.yaml"))
			assert.False(t, h.exists("kits/mise/SHASUMS256.txt"))
		})
	}
}

// TestFetchFollowsARedirectToANamedHost: the release download moves to
// a second host, as GitHub's moves to its release-assets host, and the
// run goes on when the source names that host.
func TestFetchFollowsARedirectToANamedHost(t *testing.T) {
	assets := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(fixture(t, miseSums))
	}))
	t.Cleanup(assets.Close)
	h := redirectHarness(t, func(*harness) string { return assets.URL + "/asset" })
	h.e.Mise.Hosts = append(h.e.Mise.Hosts, strings.TrimPrefix(assets.URL, "https://"))
	require.Equal(t, exitOK, h.run(t, "mise", "2026.10.3"), h.stderr.String())
	assert.Equal(t, string(fixture(t, miseSums)), h.read(t, "kits/mise/SHASUMS256.txt"))
}

// TestFetchRefusesAnHTTPSource: a source whose base is http is refused
// before any request.
func TestFetchRefusesAnHTTPSource(t *testing.T) {
	h := newHarness(t, herdrRoutes(t, "v0.9.5.json"))
	h.e.GitHub.Base = "http://" + strings.TrimPrefix(h.url, "https://")
	assert.Equal(t, exitError, h.run(t, "herdr", "0.9.5"))
	assert.Contains(t, h.stderr.String(), "GET refused: http://")
	assert.Equal(t, int32(0), h.requests.Load())
}
