// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/internal/pin"
)

const herdrPath = "/repos/herdrdev/herdr/releases/tags/v0.9.5"

// fakeEnv is an Env whose sources are one local TLS server that runs
// handler, in a new repository; no request leaves the machine.
func fakeEnv(t *testing.T, handler http.HandlerFunc) (pin.Env, *gittest.Repo) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	hosts := []string{strings.TrimPrefix(srv.URL, "https://")}
	src := pin.Source{Base: srv.URL, Hosts: hosts}
	repo := gittest.New(t)
	return pin.Env{Dir: repo.Dir, GitEnv: repo.Env, Client: srv.Client(), Mise: src, GitHub: src, Auth: src, Registry: src}, repo
}

// serve answers path with the fixture file and 404 elsewhere.
func serve(t *testing.T, path, file string) http.HandlerFunc {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", filepath.FromSlash(file)))
	require.NoError(t, err)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}
}

// TestRunUsage holds the command lines run refuses before it calls a
// command: nothing is requested and nothing is written.
func TestRunUsage(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no_command", nil, "usage:\n  go run ./tools/kitpin frontend\n"},
		{"unknown_command", []string{"gh"}, `kitpin: unknown command "gh"`},
		{"mise_without_version", []string{"mise"}, "kitpin: mise takes 1 argument(s), not 0"},
		{"mise_with_no_verify", []string{"mise", "2026.10.3", "--no-verify"}, "kitpin: mise takes 1 argument(s), not 2"},
		{"no_verify_alone", []string{"--no-verify"}, `kitpin: unknown command "--no-verify"`},
		{"frontend_with_an_argument", []string{"frontend", "3.0.0-m.8"}, "kitpin: frontend takes 0 argument(s), not 1"},
		{"workload_without_reference", []string{"workload"}, "kitpin: workload takes 1 argument(s), not 0"},
		{"herdr_with_two_versions", []string{"herdr", "0.9.4", "0.9.5"}, "kitpin: herdr takes 1 argument(s), not 2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var requests atomic.Int32
			e, repo := fakeEnv(t, func(http.ResponseWriter, *http.Request) { requests.Add(1) })
			var stdout, stderr bytes.Buffer
			assert.Equal(t, exitUsage, run(t.Context(), e, c.args, &stdout, &stderr))
			assert.Contains(t, stderr.String(), c.want)
			assert.Empty(t, stdout.String())
			assert.Zero(t, requests.Load())
			assert.NoFileExists(t, filepath.Join(repo.Dir, "kits", "pins.yaml"))
		})
	}
}

// TestRunRoutes runs each command once against the fake source: the
// line of a success goes to stdout, an error to stderr with exit 1.
func TestRunRoutes(t *testing.T) {
	cases := []struct {
		name, file   string
		args         []string
		code         int
		stdout, errs string
	}{
		{"herdr", "herdr/v0.9.5.json", []string{"herdr", "0.9.5"}, exitOK, "herdr 0.9.5: immutable release; linux-amd64 14d9e8bc", ""},
		{"herdr_refused", "herdr/mutable.json", []string{"herdr", "0.9.5"}, exitError, "", "kitpin herdr: the release v0.9.5 is not marked immutable"},
		{"workload", "herdr/v0.9.5.json", []string{"workload", "docker.io/docker/sbx-kit-claude@sha256:" + strings.Repeat("c", 64)}, exitOK, "workload: docker.io/docker/sbx-kit-claude@sha256:", ""},
		{"mise_without_key", "herdr/v0.9.5.json", []string{"mise", "2026.10.3"}, exitError, "", "kitpin mise: kits/mise-minisign.pub is missing"},
		{"frontend_without_token", "herdr/v0.9.5.json", []string{"frontend"}, exitError, "", "kitpin frontend: GET https://"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, _ := fakeEnv(t, serve(t, herdrPath, c.file))
			var stdout, stderr bytes.Buffer
			assert.Equal(t, c.code, run(t.Context(), e, c.args, &stdout, &stderr))
			if c.stdout == "" {
				assert.Empty(t, stdout.String())
			} else {
				assert.Contains(t, stdout.String(), c.stdout)
			}
			if c.errs == "" {
				assert.Empty(t, stderr.String())
			} else {
				assert.Contains(t, stderr.String(), c.errs)
			}
		})
	}
}

// TestStartInterrupted sends SIGINT while a download is in flight: the
// handler of procgroup.Main cancels the request, the run writes
// nothing, and kitpin exits 130 with the one line of an interrupted
// command (04 4.1, Signals).
func TestStartInterrupted(t *testing.T) {
	e, repo := fakeEnv(t, func(_ http.ResponseWriter, r *http.Request) {
		assert.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
		<-r.Context().Done()
	})
	var stdout, stderr bytes.Buffer
	assert.Equal(t, 130, start(e, []string{"herdr", "0.9.5"}, &stdout, &stderr))
	assert.Empty(t, stdout.String())
	assert.Equal(t, "kitpin: interrupted\n", stderr.String())
	assert.NoFileExists(t, filepath.Join(repo.Dir, "kits", "pins.yaml"))
}
