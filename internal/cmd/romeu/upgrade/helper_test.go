// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package upgrade

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// helperEnv names the role of this test binary run again as a helper
// process: gh, the gh of the tests, or swap, the swap step.
const helperEnv = "UPGRADE_TEST_HELPER"

// TestHelperProcess is not a test: the tests of this package run this
// binary again with helperEnv set, as the gh of tools.gh or to run the
// swap step, which replaces the process.
//
// As gh it records its argv and its environment in
// $HOME/gh-calls.jsonl and writes its answer to $HOME/gh-answer and its
// status to $HOME/gh-answer-exit, which the wrapper script of ghBinary
// prints and exits with: --version answers $HOME/gh-version (2.68.0
// when absent); api answers $HOME/gh-commit (40 c when absent) with the
// status of $HOME/gh-api-exit; attestation answers
// $HOME/gh-attest-output with the status of $HOME/gh-attest-exit (0
// when absent).
//
// As swap it runs the swap step on UPGRADE_TEST_SWAP_TEMP,
// UPGRADE_TEST_SWAP_EXE, the hex sha256 UPGRADE_TEST_SWAP_SUM and the
// state directory UPGRADE_TEST_SWAP_STATE, and fails when the step
// returns. As swap-refused it runs the same step, which must return an
// error, and writes the id of the error, a line feed and its message to
// UPGRADE_TEST_SWAP_RESULT; a step that replaced the process writes
// nothing.
func TestHelperProcess(t *testing.T) {
	switch os.Getenv(helperEnv) {
	case "gh":
		require.NoError(t, ghHelper(os.Args[slices.Index(os.Args, "--")+1:]))
	case "swap":
		s := swapStep{
			temp: os.Getenv("UPGRADE_TEST_SWAP_TEMP"), exe: os.Getenv("UPGRADE_TEST_SWAP_EXE"),
			stateDir: os.Getenv("UPGRADE_TEST_SWAP_STATE"), running: "v1.0.0-alpha.1",
			expires: time.Date(2026, 11, 2, 9, 1, 0, 0, time.UTC),
		}
		sum, err := hex.DecodeString(os.Getenv("UPGRADE_TEST_SWAP_SUM"))
		require.NoError(t, err)
		copy(s.sum[:], sum)
		require.NoError(t, swap(s), "the swap step returned")
	case "swap-refused":
		s := swapStep{
			temp: os.Getenv("UPGRADE_TEST_SWAP_TEMP"), exe: os.Getenv("UPGRADE_TEST_SWAP_EXE"),
			stateDir: os.Getenv("UPGRADE_TEST_SWAP_STATE"), running: "v1.0.0-alpha.1",
			expires: time.Date(2026, 11, 2, 9, 1, 0, 0, time.UTC),
		}
		sum, err := hex.DecodeString(os.Getenv("UPGRADE_TEST_SWAP_SUM"))
		require.NoError(t, err)
		copy(s.sum[:], sum)
		err = swap(s)
		require.Error(t, err, "the swap step returned no error")
		require.NoError(t, os.WriteFile(os.Getenv("UPGRADE_TEST_SWAP_RESULT"), []byte(cli.Of(err).ID+"\n"+err.Error()), 0o600))
	}
}

// ghCall is one run of the gh helper.
type ghCall struct {
	Argv []string `json:"argv"`
	Env  []string `json:"env"`
}

// shellEnv are the variables the shell of the wrapper script, and the
// script itself, add to the environment gh was given.
var shellEnv = []string{"PWD", "OLDPWD", "SHLVL", "_", helperEnv, "GORACE"}

// ghHelper is the gh helper: it records args and the environment, and
// writes the answer and the status the wrapper script gives.
func ghHelper(args []string) error {
	home := os.Getenv("HOME")
	read := func(name, def string) string {
		b, err := os.ReadFile(filepath.Join(home, name))
		if err != nil {
			return def
		}
		return string(b)
	}
	var env []string
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); !slices.Contains(shellEnv, k) {
			env = append(env, kv)
		}
	}
	slices.Sort(env)
	line, err := json.Marshal(ghCall{Argv: args, Env: env})
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(home, "gh-calls.jsonl"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	answer, exit := "", "8"
	switch {
	case len(args) == 1 && args[0] == "--version":
		v := read("gh-version", "2.68.0")
		answer, exit = "gh version "+v+" (2025-03-11)\nhttps://github.com/cli/cli/releases/tag/v"+v+"\n", "0"
	case len(args) > 0 && args[0] == "api":
		answer, exit = read("gh-commit", strings.Repeat("c", 40))+"\n", read("gh-api-exit", "0")
	case len(args) > 0 && args[0] == "attestation":
		answer, exit = read("gh-attest-output", "Loaded digest sha256 for the archive\n"), read("gh-attest-exit", "0")
	}
	if err := os.WriteFile(filepath.Join(home, "gh-answer"), []byte(answer), 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(home, "gh-answer-exit"), []byte(strings.TrimSpace(exit)), 0o600)
}

// ghBinary writes the gh of the test: a script, mode 0700 in a
// directory of its own, that runs this test binary as the gh helper
// with the arguments it was given, then prints the helper's answer and
// exits with its status. GORACE ends the sleep a race build makes
// before it exits.
func ghBinary(t *testing.T) string {
	t.Helper()
	self, err := os.Executable()
	require.NoError(t, err)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	p := filepath.Join(dir, "gh")
	script := "#!/bin/sh\n" +
		"GORACE=atexit_sleep_ms=0 " + helperEnv + "=gh '" + self + "' -test.run='^TestHelperProcess$' -- \"$@\" > /dev/null || exit 9\n" +
		"cat \"$HOME/gh-answer\"\n" +
		"exit \"$(cat \"$HOME/gh-answer-exit\")\"\n"
	require.NoError(t, os.WriteFile(p, []byte(script), 0o700))
	return p
}

// ghCalls returns the runs of the gh helper with HOME home.
func ghCalls(t *testing.T, home string) []ghCall {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, "gh-calls.jsonl"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	require.NoError(t, err)
	var calls []ghCall
	for line := range strings.Lines(string(data)) {
		var c ghCall
		require.NoError(t, json.Unmarshal([]byte(line), &c))
		calls = append(calls, c)
	}
	return calls
}

// newBinaryScript is the new romeu of the tests: a script that, run as
// the preview, records its argv and its environment under $HOME and
// prints a text that ends with Question.
const newBinaryScript = "#!/bin/sh\n" +
	"printf '%s\\n' \"$*\" >> \"$HOME/preview-argv\"\n" +
	"env | sort > \"$HOME/preview-env\"\n" +
	"printf 'the preview of the new romeu\\n%s' '" + Question + "'\n"

// archiveOf returns a tar.gz that holds the files, name to content, in
// the order given, each a regular file of mode 0755.
func archiveOf(t *testing.T, files ...[2]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: f[0], Mode: 0o755, Size: int64(len(f[1]))}))
		_, err := io.WriteString(tw, f[1])
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

// sumLine is the line of checksums.txt for data named name.
func sumLine(data []byte, name string) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) + "  " + name + "\n"
}

// request is one request a fixture server saw.
type request struct {
	Method, Host, Path, Query, Authorization string
	TLS                                      bool
}

// releaseServer serves the releases of testdata/releases-handmade.json from an
// API server, whose asset reads redirect to an asset server; both are
// TLS servers of httptest, and both record each request.
type releaseServer struct {
	api, asset *httptest.Server
	mu         gosync.Mutex
	requests   []request
	releases   []map[string]any
	// files are the contents of the assets by name; an asset without one
	// is a 404 of the asset server.
	files map[string][]byte
	// handle, when set, answers a request of the API server in place of
	// the fixture.
	handle http.HandlerFunc
}

// newReleaseServer starts the two servers with the fixture's releases
// and, for every release, the archive of runtime.GOARCH holding the new
// binary script and its checksums.txt.
func newReleaseServer(t *testing.T) *releaseServer {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "releases-handmade.json"))
	require.NoError(t, err)
	s := &releaseServer{files: map[string][]byte{}}
	require.NoError(t, json.Unmarshal(data, &s.releases))
	archive := archiveOf(t, [2]string{"COPYING", "license"}, [2]string{binaryMember, newBinaryScript})
	for _, r := range s.releases {
		tag := r["tag_name"].(string)
		name := ArchiveName(tag, runtime.GOARCH)
		s.files[name] = archive
		s.files[ChecksumsName+"@"+tag] = []byte(sumLine([]byte("other"), ArchiveName(tag, "other")) + sumLine(archive, name))
	}
	s.asset = httptest.NewTLSServer(http.HandlerFunc(s.serveAsset))
	t.Cleanup(s.asset.Close)
	s.api = httptest.NewTLSServer(http.HandlerFunc(s.serveAPI))
	t.Cleanup(s.api.Close)
	return s
}

// record keeps r.
func (s *releaseServer) record(r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, request{
		Method: r.Method, Host: r.Host, Path: r.URL.Path, Query: r.URL.RawQuery,
		Authorization: r.Header.Get("Authorization"), TLS: r.TLS != nil,
	})
}

// seen returns the requests so far.
func (s *releaseServer) seen() []request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.requests)
}

// host returns the host:port of a server.
func host(srv *httptest.Server) string { return strings.TrimPrefix(srv.URL, "https://") }

// source returns the source of the two servers, its client trusting
// their certificate.
func (s *releaseServer) source() *source {
	return newSourceWith(s.api.Client(), host(s.api), host(s.asset), "v1.0.0-alpha.1")
}

// serveAPI answers the three reads of the API: the list, a release by
// its tag, and an asset, which redirects to the asset server.
func (s *releaseServer) serveAPI(w http.ResponseWriter, r *http.Request) {
	s.record(r)
	if s.handle != nil {
		s.handle(w, r)
		return
	}
	prefix := "/repos/" + Repo + "/releases"
	rest, ok := strings.CutPrefix(r.URL.Path, prefix)
	switch {
	case !ok:
		http.NotFound(w, r)
	case rest == "" && r.URL.Query().Get("per_page") == "10":
		data, _ := json.Marshal(s.releases)
		_, _ = w.Write(data)
	case strings.HasPrefix(rest, "/tags/"):
		for _, rel := range s.releases {
			if rel["tag_name"] == strings.TrimPrefix(rest, "/tags/") {
				data, _ := json.Marshal(rel)
				_, _ = w.Write(data)
				return
			}
		}
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	case strings.HasPrefix(rest, "/assets/"):
		id := strings.TrimPrefix(rest, "/assets/")
		for _, rel := range s.releases {
			for _, a := range rel["assets"].([]any) {
				a := a.(map[string]any)
				if strconv.FormatFloat(a["id"].(float64), 'f', -1, 64) == id {
					name := a["name"].(string)
					if name == ChecksumsName {
						name += "@" + rel["tag_name"].(string)
					}
					http.Redirect(w, r, s.asset.URL+"/download/"+name, http.StatusFound)
					return
				}
			}
		}
		http.NotFound(w, r)
	default:
		http.NotFound(w, r)
	}
}

// serveAsset answers a download with the content of the asset; /hang
// answers nothing until the client goes away, or 5 s.
func (s *releaseServer) serveAsset(w http.ResponseWriter, r *http.Request) {
	s.record(r)
	if r.URL.Path == "/hang" {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
		return
	}
	data, ok := s.files[strings.TrimPrefix(r.URL.Path, "/download/")]
	if !ok {
		http.NotFound(w, r)
		return
	}
	_, _ = w.Write(data)
}

// machine is a host for the command: HOME, the state directory, the
// directory of the running romeu, host settings whose sbx is a script
// that prints a version and whose gh is the gh helper.
type machine struct {
	home, stateHome, settings, bin, exe string
	// version is the version stamp of the romeu the tests run.
	version string
}

// oldBinary is the content of the running romeu of the tests.
const oldBinary = "#!/bin/sh\necho the old romeu\n"

// newMachine writes the machine.
func newMachine(t *testing.T) machine {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	m := machine{
		home:      filepath.Join(dir, "home"),
		stateHome: filepath.Join(dir, "state"),
		settings:  filepath.Join(dir, "settings.yaml"),
		bin:       filepath.Join(dir, "bin"),
		version:   "v1.0.0-alpha.1",
	}
	m.exe = filepath.Join(m.bin, "romeu")
	for _, d := range []string{m.home, m.bin} {
		require.NoError(t, os.Mkdir(d, 0o700))
	}
	require.NoError(t, os.WriteFile(m.exe, []byte(oldBinary), 0o755))
	sbx := filepath.Join(dir, "sbx")
	require.NoError(t, os.WriteFile(sbx, []byte("#!/bin/sh\necho 'sbx version v0.49.2 0123456789abcdef'\n"), 0o700))
	m.writeSettings(t, spec.Tools{Sbx: sbx, Git: "/usr/bin/git", Gh: ghBinary(t)})
	// The state directory of a machine romeu ran on, with its lock file.
	st, err := state.Open(filepath.Join(m.stateHome, "romeu"))
	require.NoError(t, err)
	lk, err := st.Lock()
	require.NoError(t, err)
	lk.Unlock()
	require.NoError(t, st.Close())
	return m
}

// writeHome writes the file name under the HOME of m: an answer of the
// gh helper.
func (m machine) writeHome(t *testing.T, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(m.home, name), []byte(content), 0o600))
}

// writeSettings writes the host settings of m with the tools given.
func (m machine) writeSettings(t *testing.T, tools spec.Tools) {
	t.Helper()
	data, err := spec.EncodeHostSettings(spec.HostSettings{
		Schema: "host-settings/v1",
		Root:   filepath.Join(m.home, "Developer"),
		Config: spec.ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"},
		Tools:  tools,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(m.settings, data, 0o600))
}

// getenv is the environment of the machine.
func (m machine) getenv(k string) string {
	return map[string]string{"HOME": m.home, "XDG_STATE_HOME": m.stateHome}[k]
}

// use points the seams of the package at m, the reader in and the
// file descriptor errFD, the releases of srv, and a replace step that
// renames the new binary over the running one as swap does and records
// what it was given in place of replacing the process. The tests that
// call it do not run in parallel.
func use(t *testing.T, m machine, in io.Reader, errFD int, srv *releaseServer) *[]swapStep {
	t.Helper()
	oldEnv, oldIn, oldErr, oldSrc, oldReplace, oldExe := getenv, stdin, stderrFD, sources, replace, executable
	var calls []swapStep
	getenv = m.getenv
	stdin = func() io.Reader { return in }
	stderrFD = func() int { return errFD }
	sources = func(string) (*source, error) { return srv.source(), nil }
	replace = func(s swapStep) error {
		calls = append(calls, s)
		return os.Rename(s.temp, s.exe)
	}
	executable = func() (string, error) { return m.exe, nil }
	t.Cleanup(func() {
		getenv, stdin, stderrFD, sources, replace, executable = oldEnv, oldIn, oldErr, oldSrc, oldReplace, oldExe
	})
	return &calls
}

// lockedBuffer is a buffer a test reads while the command writes it.
type lockedBuffer struct {
	mu gosync.Mutex
	b  bytes.Buffer
}

// Write appends p.
func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

// String returns what was written so far.
func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// typingBuffer is the stderr under the CLI writer: it records what the
// command writes and, in the Write that shows the question, types the
// answer on the terminal through master before it returns.
type typingBuffer struct {
	lockedBuffer
	master *os.File
	answer string
	err    error
}

// Write records p and types the answer when p holds the question.
func (b *typingBuffer) Write(p []byte) (int, error) {
	n, err := b.lockedBuffer.Write(p)
	if b.answer != "" && strings.Contains(string(p), Question) {
		_, b.err = io.WriteString(b.master, b.answer)
	}
	return n, err
}

// outcome is what one run of the command gave.
type outcome struct {
	code           int
	stdout, stderr string
}

// writeToken writes the finish token of a swap into the state
// directory of m: made by the romeu of version, for the binary whose
// content is content, expiring at expires.
func (m machine) writeToken(t *testing.T, version, content string, expires time.Time) {
	t.Helper()
	require.NoError(t, writeToken(filepath.Join(m.stateHome, "romeu"),
		finishToken{version: version, sum: sha256.Sum256([]byte(content)), expires: expires}))
}

// commandDeadline is how long runCommand waits for the command to end.
const commandDeadline = 30 * time.Second

// runCommand runs romeu with args, after use, with the settings of m;
// when answer is not empty it is typed on master in the same write that
// shows the question.
func runCommand(t *testing.T, m machine, master *os.File, answer string, args ...string) outcome {
	t.Helper()
	return runCommandIn(t, t.Context(), m, master, answer, args...)
}

// runCommandIn is runCommand under the context ctx.
func runCommandIn(t *testing.T, ctx context.Context, m machine, master *os.File, answer string, args ...string) outcome {
	t.Helper()
	var stdout lockedBuffer
	stderr := &typingBuffer{master: master, answer: answer}
	env := &cli.Env{
		Binary:       "romeu",
		Stdout:       cli.NewWriter(&stdout),
		Stderr:       cli.NewWriter(stderr),
		Now:          func() time.Time { return time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC) },
		Version:      cli.Version{Version: m.version, Commit: "abc", Protocol: 1},
		SettingsPath: m.settings,
	}
	done := make(chan int, 1)
	go func() { done <- cli.Main(ctx, args, env) }()
	select {
	case code := <-done:
		require.NoError(t, stderr.err, "type the answer")
		return outcome{code: code, stdout: stdout.String(), stderr: stderr.String()}
	case <-time.After(commandDeadline):
		t.Fatalf("romeu %v did not end in %s; stderr: %q", args, commandDeadline, stderr.String())
		return outcome{}
	}
}

// closeAtEnd closes f when the test ends.
func closeAtEnd(t *testing.T, f *os.File) {
	t.Helper()
	t.Cleanup(func() {
		if err := f.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			require.NoError(t, err, "close %s", f.Name())
		}
	})
}

// node is one node of a tree as snapshot reads it: its mode and, for a
// regular file, its content.
type node struct {
	mode    fs.FileMode
	content string
}

// snapshot returns every node below each dir by its path, so a test
// compares a tree before and after a run.
func snapshot(t *testing.T, dirs ...string) map[string]node {
	t.Helper()
	out := map[string]node{}
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) && p == dir {
				return nil
			}
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			n := node{mode: info.Mode()}
			if info.Mode().IsRegular() {
				data, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				n.content = string(data)
			}
			out[p] = n
			return nil
		})
		require.NoError(t, err)
	}
	return out
}
