// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
)

// fakeRelease is a release of mise served locally over TLS: the file
// it serves at each path, and how many requests reached it.
type fakeRelease struct {
	srv   *httptest.Server
	files map[string][]byte
	hits  atomic.Int32
}

// newFakeRelease serves files, and points releaseBase and releaseRoots
// at the server for the test.
func newFakeRelease(t *testing.T, files map[string][]byte) *fakeRelease {
	t.Helper()
	r := &fakeRelease{files: files}
	r.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.hits.Add(1)
		body, ok := r.files[req.URL.Path]
		if !ok {
			http.NotFound(w, req)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(r.srv.Close)
	roots := x509.NewCertPool()
	roots.AddCert(r.srv.Certificate())
	base, prevRoots := releaseBase, releaseRoots
	releaseBase, releaseRoots = r.srv.URL+"/", roots
	t.Cleanup(func() { releaseBase, releaseRoots = base, prevRoots })
	return r
}

// hexSum is the sha256 of data in lowercase hex.
func hexSum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// installFixture is a tree whose workflow pins the fake mise of the
// release, a fake home whose search path holds an older mise, and the
// log both mises write.
type installFixture struct {
	r              *gittest.Repo
	f              citest.FakeMise
	log, onPath    string
	release, asset string
	cached, which  string
}

// newInstallFixture builds an installFixture: the release mise and the
// mise on the search path each log how they are started, and answer
// "which" with the fake go and linter, so setup can run to its end.
func newInstallFixture(t *testing.T) *installFixture {
	t.Helper()
	gitPath, err := procgroup.LookPath("git")
	require.NoError(t, err)
	x := &installFixture{r: citest.NewTree(t), f: citest.NewFakeMise(t), log: filepath.Join(t.TempDir(), "log")}
	require.NoError(t, os.Symlink(gitPath, filepath.Join(x.f.Bin, "git")))
	goCmd := x.f.Tool(t, "go", "1.27.0", "bin", "go")
	linter := x.f.Tool(t, "golangci-lint", "2.14.0", "golangci-lint")
	x.which = "if [ \"$3\" = which ]; then\ncase \"$4\" in\ngo) echo '" + goCmd + "' ;;\ngolangci-lint) echo '" + linter + "' ;;\nesac\nexit 0\nfi\n"
	x.release = "#!/bin/sh\necho \"release $*\" >> '" + x.log + "'\n" + citest.AnswerVersion + x.which
	x.onPath = filepath.Join(x.f.Bin, "mise")
	x.pathMise(t, "2026.9.15")
	pin := mise.Pin{Version: citest.FakeMiseVersion, Label: thisLabel(t)}
	x.asset = "/v" + citest.FakeMiseVersion + "/" + pin.Asset()
	x.r.Write("mise.lock", citest.LockFixture)
	x.r.Write(mise.PinnedWorkflow, citest.PinnedMiseWorkflow(t, citest.FakeMiseVersion, hexSum([]byte(x.release))))
	x.r.Commit("fixture")
	x.cached, err = mise.CachePath(citest.FakeMiseVersion)
	require.NoError(t, err)
	t.Setenv("GITHUB_ACTIONS", "")
	return x
}

// pathMise writes the mise on the search path, at version.
func (x *installFixture) pathMise(t *testing.T, version string) {
	t.Helper()
	script := "#!/bin/sh\necho \"path $*\" >> '" + x.log + "'\nif [ \"$1\" = --version ]; then echo '" + version + " linux-x64'; fi\n" + x.which
	require.NoError(t, os.WriteFile(x.onPath, []byte(script), 0o700))
}

// takeLog returns what the mises logged since the last call, and
// empties the log.
func (x *installFixture) takeLog(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(x.log)
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(t, err)
	require.NoError(t, os.Remove(x.log))
	return string(data)
}

// thisLabel is the runner label of this platform, as the pin names it.
func thisLabel(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, filepath.FromSlash(mise.PinnedWorkflow)), []byte(citest.MiseWorkflow(t, citest.FakeMiseVersion)), 0o600))
	pin, err := mise.ReadPin(root)
	require.NoError(t, err)
	return pin.Label
}

// TestSetupInstallsThePinnedMise runs setup against a fake release of
// mise served locally, one case per clause of REQ-1238.
func TestSetupInstallsThePinnedMise(t *testing.T) {
	t.Run("a file whose sha256 is not the matrix's is refused, removed and never run", func(t *testing.T) {
		x := newInstallFixture(t)
		marker := filepath.Join(t.TempDir(), "ran")
		other := []byte("#!/bin/sh\ntouch '" + marker + "'\n")
		newFakeRelease(t, map[string][]byte{x.asset: other})
		code, out := citest.Run(t, x.r, nil, Run)
		assert.Equal(t, check.ExitError, code, out)
		assert.Contains(t, out, "sha256 "+hexSum(other))
		assert.Contains(t, out, hexSum([]byte(x.release)), "the error names both hashes")
		entries, err := os.ReadDir(filepath.Dir(x.cached))
		require.NoError(t, err)
		assert.Empty(t, entries, "the download is removed")
		assert.NoFileExists(t, marker, "and never run")
		assert.Equal(t, "path --version\n", x.takeLog(t), "no other mise command ran")
	})

	t.Run("a file that matches is installed and is the mise every later mise command starts", func(t *testing.T) {
		x := newInstallFixture(t)
		release := newFakeRelease(t, map[string][]byte{x.asset: []byte(x.release)})
		code, out := citest.Run(t, x.r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
		assert.Contains(t, out, "installed mise "+citest.FakeMiseVersion+" at "+x.cached)
		assert.Equal(t, int32(1), release.hits.Load())
		info, err := os.Lstat(x.cached)
		require.NoError(t, err)
		assert.Equal(t, fs.FileMode(0o700), info.Mode(), "a regular file its owner alone can run")
		dirs, err := mise.CacheDirs(citest.FakeMiseVersion)
		require.NoError(t, err)
		for _, dir := range dirs {
			info, err := os.Lstat(dir)
			require.NoError(t, err)
			assert.Equal(t, fs.ModeDir|0o700, info.Mode(), dir)
		}
		root := citest.MustEval(t, x.r.Dir)
		assert.Equal(t, "path --version\n"+
			"release --version\n"+
			"release trust "+filepath.Join(root, "mise.toml")+"\n"+
			"release install\n"+
			"release -C "+root+" which golangci-lint\n"+
			"release -C "+root+" which go\n", x.takeLog(t), "setup starts the installed mise once it is in place")

		// fast and all start mise through these two, before any step.
		require.NoError(t, mise.CheckVersion(t.Context(), x.r.Dir))
		_, err = mise.ResolveLintTools(t.Context(), x.r.Dir)
		require.NoError(t, err)
		assert.Equal(t, "release --version\nrelease -C "+x.r.Dir+" which golangci-lint\nrelease -C "+x.r.Dir+" which go\n", x.takeLog(t))

		t.Run("a copy whose bytes changed is refused and not run, and the next setup replaces it", func(t *testing.T) {
			require.NoError(t, os.WriteFile(x.cached, []byte(x.release+"echo changed\n"), 0o700))
			err := mise.CheckVersion(t.Context(), x.r.Dir)
			require.ErrorContains(t, err, "mise is at version 2026.9.15")
			assert.Equal(t, "path --version\n", x.takeLog(t), "the changed copy never ran")
			code, out := citest.Run(t, x.r, nil, Run)
			assert.Equal(t, check.ExitOK, code, out)
			assert.Equal(t, int32(2), release.hits.Load())
			sum, err := mise.Sha256File(x.cached)
			require.NoError(t, err)
			assert.Equal(t, hexSum([]byte(x.release)), sum)
			require.NoError(t, mise.CheckVersion(t.Context(), x.r.Dir))
		})
	})

	t.Run("a download an interrupted setup left behind is removed", func(t *testing.T) {
		x := newInstallFixture(t)
		newFakeRelease(t, map[string][]byte{x.asset: []byte(x.release)})
		dirs, err := mise.CacheDirs(citest.FakeMiseVersion)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(dirs[len(dirs)-1], 0o700))
		stale := filepath.Join(dirs[len(dirs)-1], ".mise-123.download")
		require.NoError(t, os.WriteFile(stale, []byte("partial"), 0o600))
		code, out := citest.Run(t, x.r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
		entries, err := os.ReadDir(dirs[len(dirs)-1])
		require.NoError(t, err)
		require.Len(t, entries, 1, "only the installed mise is left")
		assert.Equal(t, "mise", entries[0].Name())
	})

	t.Run("a symbolic link at the cache path fails", func(t *testing.T) {
		x := newInstallFixture(t)
		release := newFakeRelease(t, map[string][]byte{x.asset: []byte(x.release)})
		target := filepath.Join(t.TempDir(), "mise")
		require.NoError(t, os.WriteFile(target, []byte(x.release), 0o700))
		dirs, err := mise.CacheDirs(citest.FakeMiseVersion)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(dirs[len(dirs)-1], 0o700))
		require.NoError(t, os.Symlink(target, x.cached))
		code, out := citest.Run(t, x.r, nil, Run)
		assert.Equal(t, check.ExitError, code, out)
		assert.Contains(t, out, x.cached+" is a symbolic link")
		assert.Equal(t, "path --version\n", x.takeLog(t), "the link is never run")
		assert.Equal(t, int32(0), release.hits.Load())
	})

	malformed := []struct{ name, version, sha, want string }{
		{"a malformed version fails", "2026.10.3.1", "", `pins the version of mise "2026.10.3.1", which is not three numbers joined by dots`},
		{"a malformed sha256 fails", citest.FakeMiseVersion, strings.Repeat("F", 64), "is not 64 lowercase hex digits"},
	}
	for _, tt := range malformed {
		t.Run(tt.name, func(t *testing.T) {
			x := newInstallFixture(t)
			release := newFakeRelease(t, map[string][]byte{x.asset: []byte(x.release)})
			sha := tt.sha
			if sha == "" {
				sha = hexSum([]byte(x.release))
			}
			x.r.Write(mise.PinnedWorkflow, citest.PinnedMiseWorkflow(t, tt.version, sha))
			x.r.Commit("a malformed pin")
			code, out := citest.Run(t, x.r, nil, Run)
			assert.Equal(t, check.ExitError, code, out)
			assert.Contains(t, out, tt.want)
			assert.Empty(t, x.takeLog(t), "no mise command ran")
			assert.Equal(t, int32(0), release.hits.Load())
		})
	}

	t.Run("a cache directory others can write fails", func(t *testing.T) {
		x := newInstallFixture(t)
		release := newFakeRelease(t, map[string][]byte{x.asset: []byte(x.release)})
		dirs, err := mise.CacheDirs(citest.FakeMiseVersion)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(dirs[0], 0o700))
		require.NoError(t, os.Chmod(dirs[0], 0o777))
		code, out := citest.Run(t, x.r, nil, Run)
		assert.Equal(t, check.ExitError, code, out)
		assert.Contains(t, out, "others can write "+dirs[0])
		assert.Equal(t, int32(0), release.hits.Load())
		assert.NoDirExists(t, dirs[1])
	})

	t.Run("a mise on the search path at the pin is used as it is, and nothing is downloaded", func(t *testing.T) {
		x := newInstallFixture(t)
		release := newFakeRelease(t, map[string][]byte{x.asset: []byte(x.release)})
		x.pathMise(t, citest.FakeMiseVersion)
		code, out := citest.Run(t, x.r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
		assert.Equal(t, int32(0), release.hits.Load())
		assert.NoFileExists(t, x.cached)
		root := citest.MustEval(t, x.r.Dir)
		assert.Equal(t, "path --version\npath --version\npath trust "+filepath.Join(root, "mise.toml")+"\npath install\npath -C "+root+" which golangci-lint\npath -C "+root+" which go\n", x.takeLog(t))
	})
}

// TestReleaseURL pins the URL of mise's binary for each of the four
// runners: the fixed prefix of REQ-1237 and the bare binary.
func TestReleaseURL(t *testing.T) {
	want := map[string]string{
		"ubuntu-26.04":     "https://github.com/jdx/mise/releases/download/v2026.10.3/mise-v2026.10.3-linux-x64",
		"ubuntu-26.04-arm": "https://github.com/jdx/mise/releases/download/v2026.10.3/mise-v2026.10.3-linux-arm64",
		"macos-26":         "https://github.com/jdx/mise/releases/download/v2026.10.3/mise-v2026.10.3-macos-arm64",
		"macos-26-intel":   "https://github.com/jdx/mise/releases/download/v2026.10.3/mise-v2026.10.3-macos-x64",
	}
	for label, url := range want {
		assert.Equal(t, url, releaseURL(mise.Pin{Version: "2026.10.3", Label: label}), label)
	}
}

// TestDownloadClient holds the HTTP client of the download to REQ-1237:
// the 15-minute limit, the proxy of the environment, TLS verification
// on, and redirects to https only.
func TestDownloadClient(t *testing.T) {
	c := downloadClient()
	assert.Equal(t, 15*time.Minute, c.Timeout)
	tr, ok := c.Transport.(*http.Transport)
	require.True(t, ok)
	assert.NotNil(t, tr.Proxy, "the proxy variables apply")
	assert.False(t, tr.TLSClientConfig.InsecureSkipVerify)
	assert.Nil(t, tr.TLSClientConfig.RootCAs, "the system roots, or those of SSL_CERT_FILE")
	assert.True(t, tr.DisableCompression, "the hash covers the bytes the server sent")
}

// TestDownload covers the download of REQ-1237 against a local server:
// a redirect is followed to https only, a status other than 200 and a
// file larger than the cap fail, and a failed download leaves no file.
func TestDownload(t *testing.T) {
	body := []byte("#!/bin/sh\n")
	x := newInstallFixture(t)
	release := newFakeRelease(t, map[string][]byte{"/file": body})
	mux := http.NewServeMux()
	mux.Handle("/file", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) }))
	mux.Handle("/to-https", http.RedirectHandler(release.srv.URL+"/file", http.StatusFound))
	plain := httptest.NewServer(mux)
	t.Cleanup(plain.Close)
	mux.Handle("/to-http", http.RedirectHandler(plain.URL+"/file", http.StatusFound))
	mux.Handle("/big", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(downloadCap+1))
		_, _ = w.Write(body)
	}))
	redirects := httptest.NewUnstartedServer(mux)
	redirects.TLS = release.srv.TLS
	redirects.StartTLS()
	t.Cleanup(redirects.Close)

	var dst bytes.Buffer
	sum, err := download(t.Context(), redirects.URL+"/to-https", &dst)
	require.NoError(t, err, "a redirect to https is followed")
	assert.Equal(t, hexSum(body), sum)
	assert.Equal(t, body, dst.Bytes())

	_, err = download(t.Context(), redirects.URL+"/to-http", &dst)
	require.ErrorContains(t, err, "a redirect to "+plain.URL+"/file leaves https")
	_, err = download(t.Context(), release.srv.URL+"/none", &dst)
	require.ErrorContains(t, err, "404 Not Found")
	_, err = download(t.Context(), redirects.URL+"/big", &dst)
	require.ErrorContains(t, err, "larger than the cap of 256 MiB")

	t.Run("the cap holds for a body without a length", func(t *testing.T) {
		var dst bytes.Buffer
		_, err := copyCapped(&dst, strings.NewReader("12345"), 4)
		require.ErrorContains(t, err, "larger than the cap of 0 MiB")
		n, err := copyCapped(&dst, strings.NewReader("1234"), 4)
		require.NoError(t, err)
		assert.Equal(t, int64(4), n)
	})

	t.Run("a failed download leaves no file", func(t *testing.T) {
		code, out := citest.Run(t, x.r, nil, Run)
		assert.Equal(t, check.ExitError, code, out)
		assert.Contains(t, out, "404 Not Found")
		entries, err := os.ReadDir(filepath.Dir(x.cached))
		require.NoError(t, err)
		assert.Empty(t, entries)
	})
}
