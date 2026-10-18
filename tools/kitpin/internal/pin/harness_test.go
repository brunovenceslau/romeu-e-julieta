// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package pin

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/pins"
)

// The fixtures are under tools/kitpin/testdata; its REUSE.toml says
// where each was recorded or how it was made.

func fixture(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", filepath.FromSlash(path)))
	require.NoError(t, err)
	return b
}

// route is one response of the fixture server.
type route struct {
	body    []byte
	headers map[string]string
	status  int // a status other than 200, as a redirect, with headers and no body
}

// harness is a repository, a fixture server, and the env that points
// kitpin at both.
type harness struct {
	repo     *gittest.Repo
	e        Env
	url      string // the server's base URL
	stdout   *bytes.Buffer
	stderr   *bytes.Buffer
	requests *atomic.Int32
}

// newHarness serves routes, by request path, and answers 404 to any
// other path. A route under /v2/ answers 401 to a request without the
// test's bearer token, as the registry does.
func newHarness(t *testing.T, routes map[string]route) *harness {
	t.Helper()
	var requests atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		rt, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v2/") && r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if rt.status != 0 {
			for k, v := range rt.headers {
				w.Header().Set(k, v)
			}
			w.WriteHeader(rt.status)
			return
		}
		for k, v := range rt.headers {
			w.Header().Set(k, v)
		}
		_, _ = w.Write(rt.body)
	}))
	t.Cleanup(srv.Close)
	host := strings.TrimPrefix(srv.URL, "https://")
	hosts := []string{host}
	h := &harness{repo: gittest.New(t), stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}, requests: &requests, url: srv.URL}
	h.e = Env{
		Dir: h.repo.Dir, GitEnv: h.repo.Env, Client: srv.Client(),
		Mise:     Source{Base: srv.URL + "/mise", Hosts: hosts},
		GitHub:   Source{Base: srv.URL, Hosts: hosts},
		Auth:     Source{Base: srv.URL + "/token", Hosts: hosts},
		Registry: Source{Base: srv.URL, Hosts: hosts},
	}
	return h
}

// The results of harness.run, as the main package maps them.
const (
	exitOK    = 0
	exitError = 1
)

// run calls the command of args as the main package does, and writes
// its line to the harness's stdout or its error to its stderr.
func (h *harness) run(t *testing.T, args ...string) int {
	t.Helper()
	ctx := t.Context()
	var done string
	var err error
	switch args[0] {
	case "frontend":
		done, err = Frontend(ctx, h.e)
	case "workload":
		done, err = Workload(ctx, h.e, args[1])
	case "herdr":
		done, err = Herdr(ctx, h.e, args[1])
	case "mise":
		done, err = Mise(ctx, h.e, args[1])
	default:
		require.Fail(t, "no command "+args[0])
	}
	if err != nil {
		_, _ = fmt.Fprintf(h.stderr, "kitpin %s: %v\n", args[0], err)
		return exitError
	}
	_, _ = fmt.Fprintln(h.stdout, done)
	return exitOK
}

func (h *harness) read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(h.repo.Dir, filepath.FromSlash(path)))
	require.NoError(t, err)
	return string(b)
}

func (h *harness) exists(path string) bool {
	_, err := os.Stat(filepath.Join(h.repo.Dir, filepath.FromSlash(path)))
	return err == nil
}

// startPins is a pin file with a herdr entry, which each command must
// keep.
const startPins = `herdr:
  version: 0.9.4
  sha256:
    linux-amd64: 493304fbefb0ed0ea0455f373b47f48d24e8774bc7974f4c916ad2e937675b23
    linux-arm64: 9511cb70982c1ef115f232058e7ed7cf9640aa5a2c0ca78934cefda3aeb093fd
`

// header is what the pin file holds before its first entry.
func header() string {
	b, err := pins.Encode(pins.Pins{})
	if err != nil {
		panic(err)
	}
	return string(b)
}
