// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e

// Package e2e holds the e2e suites of 10 10.1. This file is the entry of
// the container e2e (REQ-0744): the harness of e2e/internal/container,
// prepared once for the package, and its own cases. The cases of each
// julieta command live in e2e/julieta_<command>_test.go and those of
// each kit in e2e/kit_<name>_test.go, in this package under this tag,
// and reach the workload through workload.
package e2e

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/internal/container"
)

// The workload the container cases share, prepared by the first case
// that asks for it: the image pulled, herdr and mise downloaded and
// julieta built into binDir, which TestMain makes and removes.
var (
	prepareOnce sync.Once
	prepared    *container.Workload
	prepareErr  error
	binDir      string
	binErr      error
)

// TestMain makes binDir, which outlives each case, and removes it after
// them; a directory that cannot be made fails every case that asks for
// the workload.
func TestMain(m *testing.M) {
	binDir, binErr = os.MkdirTemp("", "container-e2e-bin-")
	code := m.Run()
	if binErr == nil {
		_ = os.RemoveAll(binDir)
	}
	os.Exit(code)
}

// config returns the harness's configuration of this repository and
// runner.
func config(t *testing.T) container.Config {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	root, err := container.ModuleRoot(wd)
	require.NoError(t, err)
	c, err := container.Load(root, runtime.GOARCH)
	require.NoError(t, err)
	return c
}

// workload returns the prepared workload, or ends the test with the
// error that stopped it: an infrastructure error for a precondition
// that could not be met (REQ-0798), a test failure for any other.
func workload(t *testing.T) *container.Workload {
	t.Helper()
	c := config(t)
	prepareOnce.Do(func() {
		prepareErr = binErr
		if prepareErr == nil {
			// The mount of the bin directory is read-only for the
			// container; the directory itself is of this user alone.
			prepared, prepareErr = container.Prepare(context.WithoutCancel(t.Context()), c, binDir)
		}
	})
	if prepareErr != nil {
		container.Fail(t, prepareErr)
	}
	return prepared
}

// versionJSON is what julieta version --json prints (04 4.1), every
// field, read strictly.
type versionJSON struct {
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	Date        string `json:"date"`
	Go          string `json:"go"`
	Protocol    int    `json:"protocol"`
	SHA256      string `json:"sha256"`
	GOARCH      string `json:"goarch"`
	BinWritable bool   `json:"binWritable"`
}

// TestContainerVersion runs julieta version --json in the workload's
// Debian base, pulled by the digest of the pin file, on the runner's
// own architecture: the binary reports the go that built it, its own
// sha256 and the runner's architecture, and sees its directory as the
// read-only mount it is.
func TestContainerVersion(t *testing.T) {
	w := workload(t)
	osRelease, err := w.Run(t.Context(), nil, "cat", "/etc/os-release")
	require.NoError(t, err)
	lines := strings.Split(string(osRelease), "\n")
	assert.True(t, slices.Contains(lines, "ID=debian") || slices.Contains(lines, `ID="debian"`), "the workload's base is Debian: %q", osRelease)

	uid, err := w.Run(t.Context(), nil, "id", "-u")
	require.NoError(t, err)
	assert.Equal(t, "65534", strings.TrimSpace(string(uid)), "the case runs as nobody, not as the image's user")

	out, err := w.Run(t.Context(), []container.Mount{container.Mount(w.Bin)}, w.Julieta, "version", "--json")
	require.NoError(t, err)
	var v versionJSON
	require.NoError(t, json.Unmarshal(out, &v, json.RejectUnknownMembers(true)), "%q", out)
	b, err := os.ReadFile(w.Julieta)
	require.NoError(t, err)
	sum := sha256.Sum256(b)
	assert.Equal(t, hex.EncodeToString(sum[:]), v.SHA256)
	assert.Equal(t, runtime.GOARCH, v.GOARCH)
	assert.Equal(t, runtime.Version(), v.Go)
	assert.Positive(t, v.Protocol)
	assert.False(t, v.BinWritable, "the bin directory is mounted read-only")
}

// TestContainerPinnedTools runs the herdr and the mise the harness
// downloaded, each checked against its pin, in the workload's base: each
// reports the version of the pin file.
func TestContainerPinnedTools(t *testing.T) {
	w := workload(t)
	for name, tool := range map[string]struct{ path, version string }{
		"herdr": {w.Herdr, w.Pins.Herdr.Version},
		"mise":  {w.Mise, w.Pins.Mise.Version},
	} {
		dir := container.Mount(w.Bin)
		out, err := w.Run(t.Context(), []container.Mount{dir}, tool.path, "--version")
		require.NoError(t, err, name)
		assert.Contains(t, string(out), tool.version, name)
	}
}

// fakeRegistry serves the distribution API on 127.0.0.1, which the
// Docker daemon of a Linux runner reaches over plain HTTP: every
// manifest request answers code, and with 200 a manifest whose content
// is not the digest the request names.
func fakeRegistry(t *testing.T, code int) string {
	t.Helper()
	manifest := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"application/vnd.oci.image.config.v1+json","digest":"sha256:` + strings.Repeat("1", 64) + `","size":2},"layers":[]}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" || code != http.StatusOK {
			if r.URL.Path != "/v2/" {
				w.WriteHeader(code)
			}
			return
		}
		_, digest, _ := strings.Cut(r.URL.Path, "/manifests/")
		w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.Header().Set("Docker-Content-Digest", digest)
		w.Header().Set("Content-Length", strconv.Itoa(len(manifest)))
		if r.Method == http.MethodGet {
			_, _ = w.Write(manifest)
		}
	}))
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

// zeroDigest names content no registry serves.
const zeroDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

// TestContainerPullTransportFailure is the fault test of the pull: a
// registry that answers 503, and a port with no listener, fail the pull
// on transport, an infrastructure error, which Fail reports with the
// prefix that the e2e step of tools/ci ends with its own exit status.
func TestContainerPullTransportFailure(t *testing.T) {
	c := config(t)
	if err := c.CheckDaemon(t.Context()); err != nil {
		container.Fail(t, err)
	}
	for name, host := range map[string]string{"a 503": fakeRegistry(t, http.StatusServiceUnavailable), "no listener": "127.0.0.1:1"} {
		err := c.PullImage(t.Context(), host+"/romeu-e-julieta/fault@"+zeroDigest)
		infra, ok := errors.AsType[*container.InfraError](err)
		require.True(t, ok, "%s: %v", name, err)
		assert.Equal(t, container.Pull, infra.Precondition, name)
		assert.True(t, strings.HasPrefix(container.Report(err), container.InfraPrefix), name)
	}
}

// TestContainerPullDigestMismatch is the fault test of the digest: a
// registry whose manifest is not the content of the digest asked for is
// a test failure, never an infrastructure error.
func TestContainerPullDigestMismatch(t *testing.T) {
	c := config(t)
	if err := c.CheckDaemon(t.Context()); err != nil {
		container.Fail(t, err)
	}
	err := c.PullImage(t.Context(), fakeRegistry(t, http.StatusOK)+"/romeu-e-julieta/fault@"+zeroDigest)
	require.Error(t, err)
	assert.NotErrorAs(t, err, new(*container.InfraError))
	assert.Contains(t, err.Error(), "not the one the pin names")
	assert.False(t, strings.HasPrefix(container.Report(err), container.InfraPrefix))
}

// TestContainerDaemonUnreachable is the fault test of the daemon, with
// the docker command of the runner: a DOCKER_HOST with no daemon behind
// it is an infrastructure error.
func TestContainerDaemonUnreachable(t *testing.T) {
	c := config(t)
	c.Env = append(c.Env, "DOCKER_HOST=unix://"+t.TempDir()+"/no.sock")
	err := c.CheckDaemon(t.Context())
	infra, ok := errors.AsType[*container.InfraError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, container.Daemon, infra.Precondition)
}
