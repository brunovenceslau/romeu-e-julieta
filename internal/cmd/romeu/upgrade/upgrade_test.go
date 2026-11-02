// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package upgrade

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// The tests of this file replace the seams of the package (use), so
// none runs in parallel.

// treeOf is what a run must leave as it was: the directory of the
// running binary, the state directory and the host settings.
func treeOf(t *testing.T, m machine) map[string]node {
	t.Helper()
	return snapshot(t, m.bin, m.stateHome, m.settings)
}

// TestUpgradeCheck holds REQ-1325: --check prints the running and the
// picked versions, and current when they are the same; it needs no
// terminal and no tools.gh, reads no host settings, makes no read but
// the one of the API, and writes nothing.
func TestUpgradeCheck(t *testing.T) {
	m := newMachine(t)
	srv := newReleaseServer(t)
	_ = use(t, m, strings.NewReader(""), -1, srv)
	require.NoError(t, os.Remove(m.settings), "no settings to read")
	before := treeOf(t, m)

	got := runCommand(t, m, nil, "", "upgrade", "--check")
	require.Equal(t, 0, got.code, got.stderr)
	assert.Equal(t, "running  v1.0.0-alpha.1\npicked   v1.0.0-alpha.2\n", got.stdout)

	got = runCommand(t, m, nil, "", "upgrade", "--check", "--version", "v1.0.0-alpha.1")
	require.Equal(t, 0, got.code, got.stderr)
	assert.Equal(t, "running  v1.0.0-alpha.1\npicked   v1.0.0-alpha.1\ncurrent\n", got.stdout)

	assert.Len(t, srv.seen(), 2, "one read of the API per run")

	srv.handle = func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "down", http.StatusBadGateway) }
	got = runCommand(t, m, nil, "", "upgrade", "--check")
	assert.Equal(t, 1, got.code)
	assert.Contains(t, got.stderr, "error RJ-362 upgrade-download-failed: ")
	assert.Empty(t, got.stdout)

	assert.Empty(t, ghCalls(t, m.home), "no gh")
	assert.Equal(t, before, treeOf(t, m))
}

// TestUpgradeBeforeAnyDownload holds REQ-1326, REQ-1334 and REQ-1335:
// without a terminal the command is the error usage; with tools.gh
// unset, not executable or below the floor it is upgrade-gh-unusable;
// with the directory of the running binary not writable it is
// upgrade-binary-unwritable; with the lock held by another romeu it is
// locked (04 4.1, Locking); each before any read of the network, and
// nothing is written.
func TestUpgradeBeforeAnyDownload(t *testing.T) {
	pipeRead, pipeWrite, err := os.Pipe()
	require.NoError(t, err)
	closeAtEnd(t, pipeRead)
	closeAtEnd(t, pipeWrite)
	for _, tt := range []struct {
		name    string
		arrange func(t *testing.T, m machine) (in io.Reader, errFD int)
		id      string
		code    int
	}{
		{"a pipe on stdin", func(t *testing.T, _ machine) (io.Reader, int) {
			t.Helper()
			_, terminal := openPTY(t)
			return pipeRead, int(terminal.Fd())
		}, "RJ-349", 2},
		{"a pipe on stderr", func(t *testing.T, _ machine) (io.Reader, int) {
			t.Helper()
			_, terminal := openPTY(t)
			return terminal, int(pipeWrite.Fd())
		}, "RJ-349", 2},
		{"tools.gh unset", func(t *testing.T, m machine) (io.Reader, int) {
			t.Helper()
			m.writeSettings(t, spec.Tools{Sbx: "/usr/bin/true", Git: "/usr/bin/git"})
			_, terminal := openPTY(t)
			return terminal, int(terminal.Fd())
		}, GhUnusable.ID, 2},
		{"tools.gh not executable", func(t *testing.T, m machine) (io.Reader, int) {
			t.Helper()
			gh := filepath.Join(m.home, "gh")
			require.NoError(t, os.WriteFile(gh, []byte("#!/bin/sh\n"), 0o600))
			m.writeSettings(t, spec.Tools{Sbx: "/usr/bin/true", Git: "/usr/bin/git", Gh: gh})
			_, terminal := openPTY(t)
			return terminal, int(terminal.Fd())
		}, GhUnusable.ID, 2},
		{"tools.gh below the floor", func(t *testing.T, m machine) (io.Reader, int) {
			t.Helper()
			m.writeHome(t, "gh-version", "2.67.0")
			_, terminal := openPTY(t)
			return terminal, int(terminal.Fd())
		}, GhUnusable.ID, 2},
		{"another romeu holds the lock", func(t *testing.T, m machine) (io.Reader, int) {
			t.Helper()
			st, err := state.Open(filepath.Join(m.stateHome, "romeu"))
			require.NoError(t, err)
			lk, err := st.Lock()
			require.NoError(t, err)
			t.Cleanup(func() { lk.Unlock(); _ = st.Close() })
			_, terminal := openPTY(t)
			return terminal, int(terminal.Fd())
		}, state.Locked.ID, state.Locked.Exit},
		{"the binary directory not writable", func(t *testing.T, _ machine) (io.Reader, int) {
			t.Helper()
			old := writable
			t.Cleanup(func() { writable = old })
			writable = func(string) error { return errors.New("permission denied") }
			_, terminal := openPTY(t)
			return terminal, int(terminal.Fd())
		}, BinaryUnwritable.ID, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newMachine(t)
			srv := newReleaseServer(t)
			in, errFD := tt.arrange(t, m)
			_ = use(t, m, in, errFD, srv)
			before := treeOf(t, m)

			got := runCommand(t, m, nil, "", "upgrade")

			assert.Equal(t, tt.code, got.code, got.stderr)
			assert.Contains(t, got.stderr, "error "+tt.id+" ")
			assert.Empty(t, srv.seen(), "no read of the network")
			assert.Equal(t, before, treeOf(t, m))
		})
	}
}

// TestUpgradeCurrent holds REQ-1324: a release equal to the running
// version prints current and exits 0, with no download and no
// attestation.
func TestUpgradeCurrent(t *testing.T) {
	m := newMachine(t)
	m.version = "v1.0.0-alpha.2"
	srv := newReleaseServer(t)
	master, terminal := openPTY(t)
	_ = use(t, m, terminal, int(terminal.Fd()), srv)
	before := treeOf(t, m)

	got := runCommand(t, m, master, "", "upgrade")

	require.Equal(t, 0, got.code, got.stderr)
	assert.Equal(t, "current  v1.0.0-alpha.2\n", got.stdout)
	assert.Len(t, srv.seen(), 1, "the pick alone")
	for _, c := range ghCalls(t, m.home) {
		assert.Equal(t, []string{"--version"}, c.Argv, "gh --version alone")
	}
	assert.Equal(t, before, treeOf(t, m))
}

// TestUpgradeDeclined holds REQ-1329 and REQ-1349: the preview's text
// and question are shown on the terminal, and any answer but y is
// upgrade-declined, exit 1; the temporary file is deleted, and the
// running binary, the state directory and the settings are as they
// were.
func TestUpgradeDeclined(t *testing.T) {
	for name, answer := range map[string]string{"n": "n\n", "yes": "yes\n", "an empty line": "\n"} {
		t.Run(name, func(t *testing.T) {
			m := newMachine(t)
			srv := newReleaseServer(t)
			master, terminal := openPTY(t)
			calls := use(t, m, terminal, int(terminal.Fd()), srv)
			before := treeOf(t, m)

			got := runCommand(t, m, master, answer, "upgrade")

			assert.Equal(t, 1, got.code, got.stderr)
			assert.Contains(t, got.stderr, "the preview of the new romeu\n"+Question)
			assert.Contains(t, got.stderr, "error RJ-366 upgrade-declined: ")
			assert.Empty(t, *calls, "no swap")
			assert.Equal(t, before, treeOf(t, m), "nothing else changed")
		})
	}
}

// TestUpgradeConfirmed holds REQ-1327, REQ-1328, REQ-1348 and REQ-1330
// on the answer y: the release is read over HTTPS from the two hosts
// with no Authorization header, checked by its sha256 and through gh
// with the argv of the contract, the new binary runs once as the
// preview with the environment of 4.1, and the swap step renames it
// over the running binary and starts it as romeu upgrade --finish
// <running version>.
func TestUpgradeConfirmed(t *testing.T) {
	m := newMachine(t)
	srv := newReleaseServer(t)
	master, terminal := openPTY(t)
	calls := use(t, m, terminal, int(terminal.Fd()), srv)

	got := runCommand(t, m, master, "y\n", "upgrade")

	require.Equal(t, 0, got.code, got.stderr)
	assert.Contains(t, got.stdout, "installing  romeu v1.0.0-alpha.2 at "+m.exe+"\n")
	require.Len(t, *calls, 1)
	assert.Equal(t, m.exe, (*calls)[0].exe)
	assert.Equal(t, []string{m.exe, "upgrade", "--finish", "v1.0.0-alpha.1"}, (*calls)[0].argv)
	assert.Equal(t, m.bin, filepath.Dir((*calls)[0].temp), "the new binary was beside the running one")
	data, err := os.ReadFile(m.exe)
	require.NoError(t, err)
	assert.Equal(t, newBinaryScript, string(data))
	entries, err := os.ReadDir(m.bin)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary file is left")

	name := ArchiveName("v1.0.0-alpha.2", runtime.GOARCH)
	var paths []string
	for _, r := range srv.seen() {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.True(t, r.TLS, "over HTTPS")
		assert.Empty(t, r.Authorization, "no Authorization header")
		assert.Contains(t, []string{host(srv.api), host(srv.asset)}, r.Host)
		paths = append(paths, r.Path)
	}
	assert.Equal(t, "/download/"+name, paths[len(paths)-1])

	gh := ghCalls(t, m.home)
	require.Len(t, gh, 3)
	assert.Equal(t, []string{"--version"}, gh[0].Argv)
	assert.Equal(t, tagCommitArgv("", "v1.0.0-alpha.2")[1:], gh[1].Argv)
	assert.Equal(t, attestArgv("", gh[2].Argv[2], strings.Repeat("c", 40))[1:], gh[2].Argv)
	assert.Equal(t, name, filepath.Base(gh[2].Argv[2]))

	argv, err := os.ReadFile(filepath.Join(m.home, "preview-argv"))
	require.NoError(t, err)
	assert.Equal(t, "upgrade --preview v1.0.0-alpha.1\n", string(argv), "the preview ran once")
	env, err := os.ReadFile(filepath.Join(m.home, "preview-env"))
	require.NoError(t, err)
	for _, want := range []string{"HOME=" + m.home, "LANG=C", "PATH=/usr/bin:/bin", "ROMEU_SETTINGS=" + m.settings, "XDG_STATE_HOME=" + m.stateHome} {
		assert.Contains(t, strings.Split(string(env), "\n"), want)
	}
}

// TestUpgradeRefusedRelease holds REQ-1332 and REQ-1333 at the command:
// a release without the archive of this arch is download-failed, an
// archive whose sha256 differs and an attestation gh refuses are
// verify-failed; each leaves the running binary and the state as they
// were and runs no preview.
func TestUpgradeRefusedRelease(t *testing.T) {
	for _, tt := range []struct {
		name    string
		arrange func(m machine, srv *releaseServer)
		id      string
	}{
		{"no archive of this arch", func(_ machine, srv *releaseServer) {
			delete(srv.files, ArchiveName("v1.0.0-alpha.2", runtime.GOARCH))
		}, DownloadFailed.ID},
		{"another sha256", func(_ machine, srv *releaseServer) {
			name := ArchiveName("v1.0.0-alpha.2", runtime.GOARCH)
			srv.files[name] = append([]byte{}, srv.files[name][:len(srv.files[name])-1]...)
		}, VerifyFailed.ID},
		{"the attestation refused", func(m machine, _ *releaseServer) {
			m.writeHome(t, "gh-attest-exit", "1")
		}, VerifyFailed.ID},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newMachine(t)
			srv := newReleaseServer(t)
			tt.arrange(m, srv)
			master, terminal := openPTY(t)
			calls := use(t, m, terminal, int(terminal.Fd()), srv)
			before := treeOf(t, m)

			got := runCommand(t, m, master, "y\n", "upgrade")

			assert.Equal(t, 1, got.code, got.stderr)
			assert.Contains(t, got.stderr, "error "+tt.id+" ")
			assert.Empty(t, *calls)
			assert.NoFileExists(t, filepath.Join(m.home, "preview-argv"), "no preview")
			assert.Equal(t, before, treeOf(t, m))
		})
	}
}

// TestUpgradeFlags: the internal forms take no other flag and a version;
// --version takes a release tag.
func TestUpgradeFlags(t *testing.T) {
	m := newMachine(t)
	srv := newReleaseServer(t)
	_ = use(t, m, strings.NewReader(""), -1, srv)
	for _, args := range [][]string{
		{"upgrade", "--preview", "v1.0.0", "--check"},
		{"upgrade", "--finish", "v1.0.0", "--version", "v1.0.0"},
		{"upgrade", "--preview", "v1.0.0", "--finish", "v1.0.0"},
		{"upgrade", "--preview", "latest"},
		{"upgrade", "--finish", "v0.0.0-dev+xyz"},
		{"upgrade", "--version", "../../x"},
		{"upgrade", "extra"},
	} {
		got := runCommand(t, m, nil, "", args...)
		assert.Equal(t, 2, got.code, "%v: %s", args, got.stderr)
		assert.Contains(t, got.stderr, "error RJ-349 usage: ", args)
	}
	assert.Empty(t, srv.seen())
	assert.True(t, runningVersion("v0.0.0-dev+0123456789abcdef-dirty"))
	assert.True(t, runningVersion("v0.0.0-dev+unknown"))
	assert.True(t, runningVersion("v1.0.0-rc.1"))
}

// TestHelpListsNoInternalFlag: the help of romeu upgrade names
// --version and --check, and neither --preview nor --finish.
func TestHelpListsNoInternalFlag(t *testing.T) {
	m := newMachine(t)
	got := runCommand(t, m, nil, "", "upgrade", "--help")
	require.Equal(t, 0, got.code, got.stderr)
	assert.Contains(t, got.stdout, "--version")
	assert.Contains(t, got.stdout, "--check")
	assert.NotContains(t, got.stdout, "--preview")
	assert.NotContains(t, got.stdout, "--finish")
}
