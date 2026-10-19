// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package container

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// TestReport checks the line Fail prints: the prefix for an
// infrastructure error, wrapped or not, and one line whatever the error
// holds, so a line break in a registry's answer cannot start a line
// with the prefix.
func TestReport(t *testing.T) {
	infra := &InfraError{Pull, errors.New("503 Service Unavailable\nsecond line")}
	assert.Equal(t, InfraPrefix+"the pull of the workload's image: 503 Service Unavailable second line", Report(infra))
	assert.Equal(t, InfraPrefix+"prepare: the Docker daemon: refused", Report(fmt.Errorf("prepare: %w", &InfraError{Daemon, errors.New("refused")})))
	assert.Equal(t, "the digest differs "+InfraPrefix+"x", Report(errors.New("the digest differs\n"+InfraPrefix+"x")))
}

// TestPullFailure classifies docker's own messages, as docker 29.8.1
// printed them on 2026-10-19 (a registry on 127.0.0.1 that answered 503,
// a port with no listener, and a manifest whose content differs from the
// digest), plus a name that does not resolve and a timeout: a transport
// failure is an infrastructure error, and a digest that differs, or a
// failure the harness does not know, is a test failure.
func TestPullFailure(t *testing.T) {
	const d = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	tests := []struct {
		name  string
		err   error
		infra bool
	}{
		{"a 5xx", errors.New(`Error response from daemon: unknown: failed to resolve reference "localhost:18555/x@` + d + `": unexpected status from HEAD request to http://localhost:18555/v2/x/manifests/` + d + `: 503 Service Unavailable`), true},
		{"a refused connection", errors.New(`Error response from daemon: failed to resolve reference "localhost:1/x@` + d + `": failed to do request: Head "https://localhost:1/v2/x/manifests/` + d + `": dial tcp [::1]:1: connect: connection refused`), true},
		{"a name that does not resolve", errors.New(`Error response from daemon: Get "https://registry-1.docker.io/v2/": dial tcp: lookup registry-1.docker.io on 127.0.0.53:53: no such host`), true},
		{"a timeout", errors.New(`Error response from daemon: Get "https://registry-1.docker.io/v2/": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)`), true},
		{"the pull limit", errors.New(`Error response from daemon: toomanyrequests: You have reached your pull rate limit`), true},
		{"the limit of the class", &procgroup.TimeoutError{Class: procgroup.ToolStep}, true},
		{"a digest that differs", errors.New(`failed commit on ref "manifest-` + d + `": commit failed: unexpected commit digest sha256:eca47388aeb57cdb4e288371a26475427d7a1920b4bfd0e7fbe6be4cbb97968f, expected ` + d + `: failed precondition`), false},
		{"a digest that differs, with a 503 in the text", errors.New(`unexpected commit digest sha256:1, expected sha256:2: 503 Service Unavailable`), false},
		{"a refusal of the proxy", errors.New(`Error response from daemon: unknown: unexpected status from HEAD request to https://x.invalid/v2/x/manifests/` + d + `: 403 Forbidden`), false},
		{"a manifest the registry does not have", errors.New(`Error response from daemon: manifest unknown`), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pullFailure(tt.err)
			_, isInfra := errors.AsType[*InfraError](got)
			assert.Equal(t, tt.infra, isInfra, "%v", got)
			assert.ErrorIs(t, got, tt.err)
		})
	}
}

// TestCheckDaemonUnreachable is the fault test of the daemon: a
// DOCKER_HOST with no daemon behind it is an infrastructure error. Where
// the docker command is not installed, the error is one too.
func TestCheckDaemonUnreachable(t *testing.T) {
	c := Config{Env: []string{"PATH=" + os.Getenv("PATH"), "DOCKER_HOST=unix://" + filepath.Join(t.TempDir(), "no.sock")}}
	err := c.CheckDaemon(t.Context())
	infra, ok := errors.AsType[*InfraError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, Daemon, infra.Precondition)
	assert.True(t, strings.HasPrefix(Report(err), InfraPrefix+Daemon+": "))
}

// release serves one release file: body with the status code.
func release(t *testing.T, code int, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func hexSum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// fixtureConfig returns a Config whose herdr and mise are pinned to the
// sha256 of content, read from base, with a cache of the test's own.
func fixtureConfig(t *testing.T, base string, content []byte) Config {
	t.Helper()
	sum := hexSum(content)
	release := &Release{Version: "1.2.3", SHA256: Arches{Amd64: sum, Arm64: sum}}
	return Config{
		Pins:      Pins{Herdr: release, Mise: release},
		Client:    http.DefaultClient,
		HerdrBase: base,
		MiseBase:  base,
		CacheDir:  filepath.Join(t.TempDir(), "cache"),
		GOARCH:    "arm64",
	}
}

// TestDownloadFails is the fault test of each download: a release host
// that answers 503, or that does not answer, is an infrastructure
// error of herdr's download and of mise's.
func TestDownloadFails(t *testing.T) {
	down := release(t, http.StatusServiceUnavailable, nil)
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	for _, base := range []string{down.URL, closed.URL} {
		c := fixtureConfig(t, base, []byte("binary"))
		for what, get := range map[string]func() (string, error){
			Herdr: func() (string, error) { return c.HerdrBinary(t.Context()) },
			Mise:  func() (string, error) { return c.MiseBinary(t.Context()) },
		} {
			_, err := get()
			infra, ok := errors.AsType[*InfraError](err)
			require.True(t, ok, "%s from %s: %v", what, base, err)
			assert.Equal(t, what, infra.Precondition)
			assert.True(t, strings.HasPrefix(Report(err), InfraPrefix+what+": "))
		}
	}
}

// TestDownloadChecksTheSHA256 checks that a downloaded file whose sha256
// is not the pin is a test failure, and so is a cached one, and that a
// file of the pin is kept under its sha256, read from the cache the next
// time under the program's own name, and read by the release's own
// asset names.
func TestDownloadChecksTheSHA256(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_, _ = w.Write([]byte("binary"))
	}))
	t.Cleanup(srv.Close)

	c := fixtureConfig(t, srv.URL, []byte("binary"))
	path, err := c.HerdrBinary(t.Context())
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(c.CacheDir, "linux-arm64", hexSum([]byte("binary")), "herdr"), path)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	_, err = c.HerdrBinary(t.Context())
	require.NoError(t, err)
	c.GOARCH = "amd64"
	_, err = c.MiseBinary(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{"/v1.2.3/herdr-linux-aarch64", "/v1.2.3/mise-v1.2.3-linux-x64"}, paths, "the second herdr is read from the cache")

	require.NoError(t, os.WriteFile(path, []byte("substituted"), 0o755))
	c.GOARCH = "arm64"
	_, err = c.HerdrBinary(t.Context())
	require.Error(t, err)
	assert.NotErrorAs(t, err, new(*InfraError))
	assert.Contains(t, err.Error(), "the cached file")

	other := fixtureConfig(t, srv.URL, []byte("another binary"))
	_, err = other.MiseBinary(t.Context())
	require.Error(t, err)
	assert.NotErrorAs(t, err, new(*InfraError))
	assert.Contains(t, err.Error(), "not the pinned")
}

// TestCacheOfThisUserAlone refuses a cache that another user may write.
func TestCacheOfThisUserAlone(t *testing.T) {
	c := fixtureConfig(t, "http://127.0.0.1:1", []byte("binary"))
	require.NoError(t, os.MkdirAll(c.CacheDir, 0o700))
	require.NoError(t, os.Chmod(c.CacheDir, 0o777))
	_, err := c.HerdrBinary(t.Context())
	require.Error(t, err)
	assert.NotErrorAs(t, err, new(*InfraError))
	assert.Contains(t, err.Error(), "only it may write")
}

// TestLoad reads the pins of this repository, which pin the workload,
// herdr and mise.
func TestLoad(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	root, err := ModuleRoot(wd)
	require.NoError(t, err)
	c, err := Load(root, "arm64")
	require.NoError(t, err)
	assert.Regexp(t, `@sha256:[0-9a-f]{64}$`, c.Pins.Workload.Ref)
	assert.Equal(t, CacheDir, c.CacheDir)

	_, err = Load(t.TempDir(), "arm64")
	require.Error(t, err)
}

// TestParsePins refuses a pin file that lacks an entry the harness
// needs or holds one in another form, and ignores the entries it does
// not read.
func TestParsePins(t *testing.T) {
	sum := strings.Repeat("a", 64)
	release := "  version: 1.2.3\n  sha256:\n    linux-amd64: " + sum + "\n    linux-arm64: " + sum + "\n"
	good := "frontend:\n  ref: x\nworkload:\n  ref: docker.io/a/b@sha256:" + sum + "\nherdr:\n" + release + "mise:\n" + release
	p, err := ParsePins([]byte(good))
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", p.Mise.Version)
	for name, src := range map[string]string{
		"no workload":       strings.Replace(good, "workload:", "other:", 1),
		"a workload by tag": strings.Replace(good, "@sha256:"+sum, ":latest", 1),
		"no mise":           strings.Replace(good, "mise:", "other2:", 1),
		"a short sha256":    strings.Replace(good, "linux-arm64: "+sum, "linux-arm64: abc", 1),
		"not YAML":          "workload: [",
	} {
		_, err := ParsePins([]byte(src))
		assert.Error(t, err, name)
	}
}
