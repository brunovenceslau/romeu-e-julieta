// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package container

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/moduleroot"
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
		{"the deadline of the pull", fmt.Errorf("docker pull: %w", context.DeadlineExceeded), true},
		{"a digest that differs", errors.New(`failed commit on ref "manifest-` + d + `": commit failed: unexpected commit digest sha256:eca47388aeb57cdb4e288371a26475427d7a1920b4bfd0e7fbe6be4cbb97968f, expected ` + d + `: failed precondition`), false},
		{"a digest that differs, with a 503 in the text", errors.New(`unexpected commit digest sha256:1, expected sha256:2: 503 Service Unavailable`), false},
		{"a layer that fails its check", errors.New(`Error response from daemon: filesystem layer verification failed for digest sha256:` + strings.Repeat("1", 64)), false},
		{"another sha256 beside a refused connection", errors.New(`pull sha256:` + strings.Repeat("2", 64) + `: dial tcp [::1]:1: connect: connection refused`), true},
		{"a 503 on a layer fetch", errors.New(`Error response from daemon: unknown: failed to copy: httpReadSeeker: failed open: unexpected status from GET request to https://registry-1.docker.io/v2/docker/sbx-kit-claude/blobs/sha256:` + strings.Repeat("3", 64) + `: 503 Service Unavailable`), true},
		{"a 429 on a layer fetch", errors.New(`Error response from daemon: unexpected status from GET request to https://registry-1.docker.io/v2/x/blobs/sha256:` + strings.Repeat("4", 64) + `: 429 Too Many Requests`), true},
		{"a mismatch beside a timeout", errors.New(`size mismatch: dial tcp 10.0.0.1:443: i/o timeout`), false},
		{"a timeout in other words", errors.New(`Error response from daemon: some timeout happened`), false},
		{"a dial that timed out", errors.New(`Error response from daemon: Get "https://registry-1.docker.io/v2/": dial tcp 10.0.0.1:443: i/o timeout`), true},
		{"a refusal of the proxy", errors.New(`Error response from daemon: unknown: unexpected status from HEAD request to https://x.invalid/v2/x/manifests/` + d + `: 403 Forbidden`), false},
		{"a manifest the registry does not have", errors.New(`Error response from daemon: manifest unknown`), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pullFailure(tt.err, d)
			_, isInfra := errors.AsType[*InfraError](got)
			assert.Equal(t, tt.infra, isInfra, "%v", got)
			assert.ErrorIs(t, got, tt.err)
		})
	}
}

// TestRunArgs holds the docker run of the cases to its hardening: no
// network, no capability, no new privileges, a bound on processes and
// memory, the user nobody, and each mount read-only at its own path.
func TestRunArgs(t *testing.T) {
	args, err := runArgs("img@sha256:x", []Mount{"/bin dir"}, []string{"/bin dir/julieta", "version", "--json"})
	require.NoError(t, err)
	assert.Equal(t, []string{"run", "--rm", "--pull=never", "--network=none",
		"--cap-drop=ALL", "--security-opt=no-new-privileges", "--pids-limit=256", "--memory=1g", "--user=65534:65534",
		"--mount", "type=bind,source=/bin dir,target=/bin dir,readonly",
		"--entrypoint", "/bin dir/julieta", "img@sha256:x", "version", "--json"}, args)
	_, err = runArgs("img", []Mount{"/a,b"}, []string{"x"})
	require.Error(t, err)
	_, err = runArgs("img", nil, nil)
	require.Error(t, err)
}

// TestFailRecordsAnInfrastructureError checks that Fail names a test
// that met an infrastructure error in the file of InfraFileEnv, and no
// test that failed otherwise, and that it never makes the file.
func TestFailRecordsAnInfrastructureError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "infra")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	t.Setenv(InfraFileEnv, path)
	require.NoError(t, recordInfra("TestA/sub"))
	require.NoError(t, recordInfra("TestB"))
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "TestA/sub\nTestB\n", string(got))

	t.Setenv(InfraFileEnv, filepath.Join(t.TempDir(), "missing"))
	require.Error(t, recordInfra("TestC"), "the file is the step's, never the harness's")
	t.Setenv(InfraFileEnv, "")
	require.NoError(t, recordInfra("TestD"), "a run without the step records nothing")
}

// TestNoticeIsRecordedInTheStepsFile checks that notice appends one line
// to the file of NoticeFileEnv, with the line breaks of the message
// turned into spaces, that it never makes the file, and that a run
// without the step prints the line to the standard error instead.
func TestNoticeIsRecordedInTheStepsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notices")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	t.Setenv(NoticeFileEnv, path)
	require.NoError(t, notice("first\n::warning::forged\r\nsecond"))
	require.NoError(t, notice("third"))
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "first ::warning::forged second\nthird\n", string(got), "one line for each notice")

	t.Setenv(NoticeFileEnv, filepath.Join(t.TempDir(), "missing"))
	require.Error(t, notice("lost"), "the file is the step's, never the harness's")

	t.Setenv(NoticeFileEnv, "")
	var stderr bytes.Buffer
	defer func(w io.Writer) { noticeFallback = w }(noticeFallback)
	noticeFallback = &stderr
	require.NoError(t, notice("local"))
	assert.Equal(t, "container e2e: notice: local\n", stderr.String())
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

// noticeFile makes the step's file of notices, as tools/ci does, names
// it in NoticeFileEnv for the test, and returns its path.
func noticeFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "notices")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	t.Setenv(NoticeFileEnv, path)
	return path
}

// readNotices returns the lines of the file of notices.
func readNotices(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
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
			Herdr: func() (string, error) { return c.HerdrBinary(t.Context(), t.TempDir()) },
			Mise:  func() (string, error) { return c.MiseBinary(t.Context(), t.TempDir()) },
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
// is not the pin is a test failure, and a corrupt cached one is fetched
// once more; and that a file of the pin is kept in the cache under its sha256, read from there the next time,
// installed under the program's own name, and read by the release's own
// asset names.
func TestDownloadChecksTheSHA256(t *testing.T) {
	notices := noticeFile(t)
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_, _ = w.Write([]byte("binary"))
	}))
	t.Cleanup(srv.Close)

	c := fixtureConfig(t, srv.URL, []byte("binary"))
	bin := t.TempDir()
	path, err := c.HerdrBinary(t.Context(), bin)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(bin, "herdr"), path)
	cached := filepath.Join(c.CacheDir, "linux-arm64", hexSum([]byte("binary")), "herdr")
	assert.FileExists(t, cached)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "binary", string(got))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	_, err = c.HerdrBinary(t.Context(), t.TempDir())
	require.NoError(t, err)
	c.GOARCH = "amd64"
	_, err = c.MiseBinary(t.Context(), t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, []string{"/v1.2.3/herdr-linux-aarch64", "/v1.2.3/mise-v1.2.3-linux-x64"}, paths, "the second herdr is read from the cache")

	assert.Equal(t, []string{""}, readNotices(t, notices), "a download and a cached read leave no notice")

	// A corrupt entry is deleted and fetched once more.
	require.NoError(t, os.WriteFile(cached, []byte("substituted"), 0o755))
	c.GOARCH = "arm64"
	bin = t.TempDir()
	paths = nil
	path, err = c.HerdrBinary(t.Context(), bin)
	require.NoError(t, err)
	assert.Equal(t, []string{"/v1.2.3/herdr-linux-aarch64"}, paths, "the corrupt entry is fetched once")
	got, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "binary", string(got))
	got, err = os.ReadFile(cached)
	require.NoError(t, err)
	assert.Equal(t, "binary", string(got), "the cache holds the fetched file")
	assert.Equal(t, []string{
		Herdr + ": the cached file " + cached + " has the sha256 " + hexSum([]byte("substituted")) + ", not the pinned " + hexSum([]byte("binary")) + "; it was removed and fetched once more",
	}, readNotices(t, notices), "the removal names the file and the pin")

	other := fixtureConfig(t, srv.URL, []byte("another binary"))
	_, err = other.MiseBinary(t.Context(), t.TempDir())
	require.Error(t, err)
	assert.NotErrorAs(t, err, new(*InfraError))
	assert.Contains(t, err.Error(), "not the pinned")
}

// TestCorruptEntryFetchedOnceStillChecked checks that a corrupt cached
// entry whose fetched replacement also fails the pin stays a test
// failure, after one fetch, and installs nothing.
func TestCorruptEntryFetchedOnceStillChecked(t *testing.T) {
	noticeFile(t)
	var fetches int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches++
		_, _ = w.Write([]byte("substituted again"))
	}))
	t.Cleanup(srv.Close)
	c := fixtureConfig(t, srv.URL, []byte("binary"))
	dir := filepath.Join(c.CacheDir, "linux-arm64", hexSum([]byte("binary")))
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "herdr"), []byte("substituted"), 0o755))
	bin := t.TempDir()
	_, err := c.HerdrBinary(t.Context(), bin)
	require.Error(t, err)
	assert.NotErrorAs(t, err, new(*InfraError))
	assert.Contains(t, err.Error(), "not the pinned")
	assert.Equal(t, 1, fetches)
	assert.NoFileExists(t, filepath.Join(bin, "herdr"))
	assert.NoFileExists(t, filepath.Join(dir, "herdr"), "nothing unchecked is cached")
}

// TestCorruptEntryRetryFails checks the two ways the second look at a
// corrupt entry can go wrong: the entry cannot be removed, which is a
// test failure that says so, and the fetch that follows fails at
// transport, which is the infrastructure error of the download; and the
// removal cannot be recorded, which is a test failure before any fetch.
func TestCorruptEntryRetryFails(t *testing.T) {
	corrupt := func(t *testing.T, c Config) string {
		t.Helper()
		dir := filepath.Join(c.CacheDir, "linux-arm64", hexSum([]byte("binary")))
		require.NoError(t, os.MkdirAll(dir, 0o700))
		path := filepath.Join(dir, "herdr")
		require.NoError(t, os.WriteFile(path, []byte("substituted"), 0o755))
		return dir
	}
	t.Run("the entry cannot be removed", func(t *testing.T) {
		require.NotZero(t, os.Getuid(), "root removes from a directory it cannot write")
		c := fixtureConfig(t, "http://127.0.0.1:1", []byte("binary"))
		dir := corrupt(t, c)
		require.NoError(t, os.Chmod(dir, 0o500))
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
		_, err := c.HerdrBinary(t.Context(), t.TempDir())
		require.Error(t, err)
		assert.NotErrorAs(t, err, new(*InfraError))
		assert.Contains(t, err.Error(), "could not be removed")
	})
	t.Run("the removal cannot be recorded", func(t *testing.T) {
		t.Setenv(NoticeFileEnv, filepath.Join(t.TempDir(), "missing"))
		var fetches int
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { fetches++ }))
		t.Cleanup(srv.Close)
		c := fixtureConfig(t, srv.URL, []byte("binary"))
		corrupt(t, c)
		_, err := c.HerdrBinary(t.Context(), t.TempDir())
		require.Error(t, err)
		assert.NotErrorAs(t, err, new(*InfraError))
		assert.Contains(t, err.Error(), "could not be recorded")
		assert.Zero(t, fetches, "a heal nobody can see is not made")
	})
	t.Run("the fetch fails at transport", func(t *testing.T) {
		noticeFile(t)
		closed := httptest.NewServer(http.NotFoundHandler())
		closed.Close()
		c := fixtureConfig(t, closed.URL, []byte("binary"))
		corrupt(t, c)
		_, err := c.HerdrBinary(t.Context(), t.TempDir())
		infra, ok := errors.AsType[*InfraError](err)
		require.True(t, ok, "%v", err)
		assert.Equal(t, Herdr, infra.Precondition)
	})
}

// TestCacheOfThisUserAlone refuses a cache that another user may write.
func TestCacheOfThisUserAlone(t *testing.T) {
	c := fixtureConfig(t, "http://127.0.0.1:1", []byte("binary"))
	require.NoError(t, os.MkdirAll(c.CacheDir, 0o700))
	require.NoError(t, os.Chmod(c.CacheDir, 0o777))
	_, err := c.HerdrBinary(t.Context(), t.TempDir())
	require.Error(t, err)
	assert.NotErrorAs(t, err, new(*InfraError))
	assert.Contains(t, err.Error(), "only it may write")
}

// TestLoad reads the pins of this repository, which pin the workload,
// herdr and mise.
func TestLoad(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	root, err := moduleroot.Find(wd)
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
