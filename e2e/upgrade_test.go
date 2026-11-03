// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e && (linux || darwin)

package e2e

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/internal/romeu"
	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios"
	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios/j9"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
)

// The release the e2e of romeu upgrade serves: the running romeu is
// stamped upgradeRunning, and the one published release is upgradeTag.
const (
	upgradeRunning = "v1.0.0-alpha.1"
	upgradeTag     = "v1.0.0-alpha.2"
	// upgradePackage is the package whose injected values (10 10.4) the
	// running romeu is built with.
	upgradePackage = "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/upgrade"
	// upgradeCommit is the commit the gh stub prints for the tag.
	upgradeCommit = "0123456789abcdef0123456789abcdef01234567"
)

// upgradeFixture is a machine whose romeu is a plain build stamped
// upgradeRunning, with the releases API and the asset host injected as
// two TLS servers of this test, which serve one release whose darwin
// archive of runtime.GOARCH holds the romeu of the host-native embedded
// build of tools/release (T103): the new binary. tools.gh is a stub at
// the floor that logs its argv; sbx is the fake.
type upgradeFixture struct {
	e *romeu.Env
	// newBinary is the content of the new romeu, and exe the path of
	// the running one, in a directory of its own.
	newBinary []byte
	exe       string
	// ghLog is the file the gh stub appends its argv to.
	ghLog string
	// requests are the paths the two servers were asked, in order.
	requests *paths
}

// paths is a list of request paths the servers append to while the
// test reads it.
type paths struct {
	mu   sync.Mutex
	list []string
}

// add appends p.
func (l *paths) add(p string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list = append(l.list, p)
}

// all returns the paths so far.
func (l *paths) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.list...)
}

// newUpgradeFixture builds the machine.
func newUpgradeFixture(t *testing.T) upgradeFixture {
	t.Helper()
	e := romeu.NewEmbedded(t, nil)
	newBinary, err := os.ReadFile(e.Romeu)
	require.NoError(t, err)
	tmp := filepath.Dir(e.Home)
	f := upgradeFixture{e: e, newBinary: newBinary, ghLog: filepath.Join(tmp, "gh.log"), requests: &paths{}}

	archiveName := "romeu_" + upgradeTag + "_darwin_" + runtime.GOARCH + ".tar.gz"
	archive := tarGz(t, map[string][]byte{"romeu": newBinary})
	sum := sha256.Sum256(archive)
	files := map[string][]byte{
		archiveName:     archive,
		"checksums.txt": []byte(hex.EncodeToString(sum[:]) + "  " + archiveName + "\n"),
	}
	asset := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.add(r.URL.Path)
		data, ok := files[strings.TrimPrefix(r.URL.Path, "/a/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(asset.Close)
	releases, err := json.Marshal([]map[string]any{{
		"tag_name": upgradeTag, "draft": false, "prerelease": true,
		"assets": []map[string]any{{"id": 11, "name": archiveName}, {"id": 12, "name": "checksums.txt"}},
	}})
	require.NoError(t, err)
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.add(r.URL.Path)
		prefix := "/repos/brunovenceslau/romeu-e-julieta/releases"
		switch r.URL.Path {
		case prefix:
			_, _ = w.Write(releases)
		case prefix + "/assets/11":
			http.Redirect(w, r, asset.URL+"/a/"+archiveName, http.StatusFound)
		case prefix + "/assets/12":
			http.Redirect(w, r, asset.URL+"/a/checksums.txt", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)

	ldflags := strings.Join([]string{
		"-X main.version=" + upgradeRunning,
		"-X " + upgradePackage + ".apiHost=" + strings.TrimPrefix(api.URL, "https://"),
		"-X " + upgradePackage + ".assetHost=" + strings.TrimPrefix(asset.URL, "https://"),
		"-X " + upgradePackage + ".rootCA=" + base64.StdEncoding.EncodeToString(api.Certificate().Raw),
	}, " ")
	f.exe = buildRomeu(t, ldflags)
	e.Romeu = f.exe

	ghDir := filepath.Join(tmp, "ghbin")
	require.NoError(t, os.Mkdir(ghDir, 0o700))
	gh := filepath.Join(ghDir, "gh")
	stub := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + f.ghLog + "\n" +
		"case \"$1\" in\n" +
		"--version) echo 'gh version 2.68.0 (2025-03-11)' ;;\n" +
		"api) echo " + upgradeCommit + " ;;\n" +
		"attestation) echo 'Loaded digest sha256 for the archive' ;;\n" +
		"*) exit 9 ;;\n" +
		"esac\n"
	require.NoError(t, os.WriteFile(gh, []byte(stub), 0o700))
	h := all.HostSettings{Config: all.ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"}}
	h.Tools.Gh = gh
	e.WriteSettings(t, h)
	session, err := filepath.Abs(filepath.Join("testdata", "sbx", "0.49.2", "handshake.jsonl"))
	require.NoError(t, err)
	require.NoError(t, e.SetSbxSession(session))
	// The machine runs the old romeu, acknowledged by gate 1.
	res := e.RunTerminal(t, question, "y\n", approveArgs)
	require.Equal(t, 0, res.Code, res.Stderr)
	return f
}

// tarGz returns a tar.gz of the files, each a regular file of mode
// 0755.
func tarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, data := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: name, Mode: 0o755, Size: int64(len(data))}))
		_, err := tw.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

// tree returns what a declined upgrade must leave as it was: the
// directory of the running romeu, HOME, the settings and the state.
func (f upgradeFixture) tree(t *testing.T) map[string]romeu.Node {
	t.Helper()
	out := map[string]romeu.Node{}
	for _, dir := range []string{filepath.Dir(f.exe), f.e.Home, f.e.ConfigHome, f.e.StateHome} {
		for p, n := range romeu.Snapshot(t, dir) {
			out[filepath.Join(dir, p)] = n
		}
	}
	return out
}

// j9Line returns the first line of J9 and its arguments after the
// binary.
func j9Line(t *testing.T, newSHA string) (scenarios.Scenario, []string) {
	t.Helper()
	tbl, err := scenarios.Load("j9")
	require.NoError(t, err)
	s := j9.J9(tbl, j9.Vars{NewSHA256: newSHA})
	line := s.Lines()[0]
	return s, strings.Fields(line)[1:]
}

// TestUpgradeCheck: romeu upgrade --check needs no terminal and prints
// the running and the picked versions (REQ-1325).
func TestUpgradeCheck(t *testing.T) {
	t.Parallel()
	f := newUpgradeFixture(t)

	res := f.e.Run(t, []string{"upgrade", "--check"})

	require.Equal(t, 0, res.Code, res.Stderr)
	assert.Equal(t, "running  "+upgradeRunning+"\npicked   "+upgradeTag+"\n", res.Stdout)
	assert.NoFileExists(t, f.ghLog, "no gh")
}

// TestUpgradeNeedsATerminal: without a terminal romeu upgrade is the
// error usage, before any read of the network (REQ-1326).
func TestUpgradeNeedsATerminal(t *testing.T) {
	t.Parallel()
	f := newUpgradeFixture(t)
	_, args := j9Line(t, "")

	res := f.e.Run(t, args)

	assert.Equal(t, 2, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "error RJ-349 usage: no-tty: ")
	assert.Empty(t, f.requests.all())
}

// TestUpgradeDeclined is J9's first step answered n: the preview of the
// new binary is shown with the one question, the answer is
// upgrade-declined, exit 1, and an fs snapshot diff shows nothing
// changed (REQ-1329, REQ-1349).
func TestUpgradeDeclined(t *testing.T) {
	t.Parallel()
	f := newUpgradeFixture(t)
	_, args := j9Line(t, "")
	before := f.tree(t)

	res := f.e.RunTerminal(t, j9.Question, "n\n", args)

	assert.Equal(t, 1, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "gate 1: ")
	assert.Contains(t, res.Stderr, "romeu "+upgradeRunning+" -> v0.0.0-dev+", "the toolchain change of approve")
	assert.Contains(t, res.Stderr, j9.Question)
	assert.Contains(t, res.Stderr, "error RJ-366 upgrade-declined: ")
	assert.Equal(t, before, f.tree(t), "nothing changed")
}

// TestUpgradeSwapsAndFinishes is J9's first step answered y: the new
// binary is renamed over the running one and replaces the process,
// which records gate 1 with no second prompt and runs the steps of sync
// --all; with no clone of the config repo on this machine, sync stops at
// its step 2 (config-clone-missing, exit 2), after gate 1 passed at its
// step 1 (REQ-1330), the finish token of the swap consumed. gh ran with
// the argv of the verification contract.
func TestUpgradeSwapsAndFinishes(t *testing.T) {
	t.Parallel()
	f := newUpgradeFixture(t)
	sum := sha256.Sum256(f.newBinary)
	s, args := j9Line(t, hex.EncodeToString(sum[:]))

	res := f.e.RunTerminal(t, j9.Question, "y\n", args)

	assert.Equal(t, 2, res.Code, res.Stderr)
	assert.Equal(t, 1, strings.Count(res.Stderr, "[y/N]"), "one prompt: %s", res.Stderr)
	assert.Contains(t, res.Stderr, "config-clone-missing", "the steps of sync --all ran past gate 1")
	assert.NotContains(t, res.Stderr, "toolchain-approval-required")
	assert.Contains(t, res.Stdout, "installing  romeu "+upgradeTag+" at "+f.exe+"\n")
	assert.Contains(t, res.Stdout, "upgraded      romeu "+upgradeRunning+" -> v0.0.0-dev+")
	got, err := os.ReadFile(f.exe)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(f.newBinary, got), "the new binary is in place of the running one")
	entries, err := os.ReadDir(filepath.Dir(f.exe))
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary file is left")
	require.NoError(t, s.Steps[0].Assert(f.e.World(t)))
	assert.FileExists(t, filepath.Join(f.e.StateDir(), "toolchain.json"), "the finish passed the token of the swap")
	assert.NoFileExists(t, filepath.Join(f.e.StateDir(), "upgrade-finish"), "the finish consumed the token of the swap")

	log, err := os.ReadFile(f.ghLog)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(log)), "\n")
	require.Len(t, lines, 3, string(log))
	assert.Equal(t, "--version", lines[0])
	assert.Equal(t, "api repos/brunovenceslau/romeu-e-julieta/commits/"+upgradeTag+" --jq .sha", lines[1])
	assert.Regexp(t, `^attestation verify /\S+/romeu_`+upgradeTag+`_darwin_`+runtime.GOARCH+`\.tar\.gz --repo brunovenceslau/romeu-e-julieta `+
		`--signer-workflow brunovenceslau/romeu-e-julieta/\.github/workflows/release\.yml --source-digest `+upgradeCommit+
		` --source-ref refs/heads/main --deny-self-hosted-runners$`, lines[2])
}
