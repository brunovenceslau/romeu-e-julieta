// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package approve

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// closeAtEnd closes f when the test ends.
func closeAtEnd(t *testing.T, f *os.File) {
	t.Helper()
	t.Cleanup(func() {
		if err := f.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			require.NoError(t, err, "close %s", f.Name())
		}
	})
}

// lockedBuffer is a buffer a test reads while the command writes it.
type lockedBuffer struct {
	mu sync.Mutex
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

// machine is a host for the command: HOME, the state directory, host
// settings whose sbx is a script that prints a version and counts its
// calls.
type machine struct {
	home, stateHome, settings, calls string
}

// newMachine writes the settings and the sbx script.
func newMachine(t *testing.T) machine {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	m := machine{
		home:      filepath.Join(dir, "home"),
		stateHome: filepath.Join(dir, "state"),
		settings:  filepath.Join(dir, "settings.yaml"),
		calls:     filepath.Join(dir, "sbx-calls"),
	}
	require.NoError(t, os.Mkdir(m.home, 0o700))
	sbx := filepath.Join(dir, "sbx")
	script := "#!/bin/sh\necho \"$*\" >> " + m.calls + "\necho 'sbx version v0.49.2 0123456789abcdef'\n"
	require.NoError(t, os.WriteFile(sbx, []byte(script), 0o700))
	data, err := spec.EncodeHostSettings(spec.HostSettings{
		Schema: "host-settings/v1",
		Root:   filepath.Join(m.home, "dev"),
		Config: spec.ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"},
		Tools:  spec.Tools{Sbx: sbx, Git: "/usr/bin/git"},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(m.settings, data, 0o600))
	return m
}

// sbxCalls returns the argv lines the sbx script was given.
func (m machine) sbxCalls(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(m.calls)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	require.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// record reads toolchain.json, nil when it is absent.
func (m machine) record(t *testing.T) *state.Toolchain {
	t.Helper()
	st, err := state.Open(filepath.Join(m.stateHome, "romeu"))
	require.NoError(t, err)
	defer func() { _ = st.Close() }()
	rec, err := Load(st)
	require.NoError(t, err)
	return rec
}

// outcome is what one run of the command gave.
type outcome struct {
	code           int
	stdout, stderr string
}

// approveOn runs romeu approve --toolchain on m with in as its standard
// input. When answer is not empty, it is typed on master once the
// question is on stderr.
func approveOn(t *testing.T, m machine, in io.Reader, master *os.File, answer string) outcome {
	t.Helper()
	oldEnv, oldIn := getenv, stdin
	getenv = func(k string) string {
		return map[string]string{"HOME": m.home, "XDG_STATE_HOME": m.stateHome}[k]
	}
	stdin = func() io.Reader { return in }
	t.Cleanup(func() { getenv, stdin = oldEnv, oldIn })

	var stdout, stderr lockedBuffer
	env := &cli.Env{
		Binary:       "romeu",
		Stdout:       cli.NewWriter(&stdout),
		Stderr:       cli.NewWriter(&stderr),
		Now:          func() time.Time { return time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC) },
		Version:      cli.Version{Version: "v1.0.0", Commit: "abc", Protocol: 1},
		SettingsPath: m.settings,
	}
	done := make(chan int, 1)
	go func() { done <- cli.Main(t.Context(), []string{"approve", "--toolchain"}, env) }()
	if answer != "" {
		deadline := time.Now().Add(10 * time.Second)
		for !strings.Contains(stderr.String(), Question) {
			require.True(t, time.Now().Before(deadline), "the question never came; stderr: %q", stderr.String())
			time.Sleep(5 * time.Millisecond)
		}
		_, err := io.WriteString(master, answer)
		require.NoError(t, err)
	}
	code := <-done
	return outcome{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// TestApproveNeedsATerminal: without a terminal on stdin the command
// exits 2 before it calls sbx or writes anything (04 4.2, REQ-0310);
// stdin decides the TTY, so a terminal elsewhere does not count.
//
//romeu:invariant I7 test
func TestApproveNeedsATerminal(t *testing.T) {
	pipeRead, pipeWrite, err := os.Pipe()
	require.NoError(t, err)
	closeAtEnd(t, pipeRead)
	closeAtEnd(t, pipeWrite)
	_, err = io.WriteString(pipeWrite, "y\n")
	require.NoError(t, err)
	for _, tt := range []struct {
		name string
		in   io.Reader
	}{
		{"a pipe", pipeRead},
		{"a reader that is no file", strings.NewReader("y\n")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newMachine(t)
			got := approveOn(t, m, tt.in, nil, "")
			assert.Equal(t, 2, got.code)
			assert.Contains(t, got.stderr, "RJ-349")
			assert.Contains(t, got.stderr, "no-tty")
			assert.Empty(t, m.sbxCalls(t), "no sbx call without a terminal")
			assert.Nil(t, m.record(t))
		})
	}
}

// TestApproveOnATerminal: on a terminal the command prints gate 1 to
// stderr, writes toolchain.json on y, and on the next run finds nothing
// to acknowledge; a declined prompt writes nothing.
//
//romeu:invariant I7 test
func TestApproveOnATerminal(t *testing.T) {
	t.Run("declined", func(t *testing.T) {
		m := newMachine(t)
		master, terminal := openPTY(t)
		got := approveOn(t, m, terminal, master, "n\n")
		assert.Equal(t, 0, got.code, got.stderr)
		assert.Contains(t, got.stderr, "gate 1: the toolchain of this machine\n")
		assert.Contains(t, got.stderr, "declined: toolchain.json is unchanged")
		assert.Nil(t, m.record(t))
	})
	t.Run("approved, then acknowledged already", func(t *testing.T) {
		m := newMachine(t)
		master, terminal := openPTY(t)
		got := approveOn(t, m, terminal, master, "y\n")
		require.Equal(t, 0, got.code, got.stderr)
		assert.Contains(t, got.stderr, "  sbx 0.49.2 (none recorded) (newest recorded 0.49.2; tested window 0.49.2)\n")
		assert.Contains(t, got.stdout, "acknowledged  "+filepath.Join(m.stateHome, "romeu", "toolchain.json"))
		assert.Empty(t, strings.TrimPrefix(got.stdout, "acknowledged  "+filepath.Join(m.stateHome, "romeu", "toolchain.json")+"\n"),
			"the prompt goes to stderr, never stdout")
		rec := m.record(t)
		require.NotNil(t, rec)
		assert.Equal(t, "0.49.2", rec.SbxVersion)
		assert.Equal(t, "v1.0.0", rec.Romeu.Version)
		assert.Equal(t, time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC), rec.AcknowledgedAt)
		assert.Equal(t, []string{"version"}, m.sbxCalls(t), "gate 1 reads sbx version and calls nothing else")

		again := approveOn(t, m, terminal, master, "")
		assert.Equal(t, 0, again.code, again.stderr)
		assert.Contains(t, again.stdout, "acknowledged already")
		assert.Equal(t, rec, m.record(t), "a second run writes nothing")
	})
}

// TestCheckFunc: the seam the preflight and sync call checks the live
// toolchain, sbx version read through the runner, against the record.
//
//romeu:invariant I7 test
func TestCheckFunc(t *testing.T) {
	m := newMachine(t)
	master, terminal := openPTY(t)
	h, err := spec.DecodeHostSettings(mustRead(t, m.settings))
	require.NoError(t, err)
	st, err := state.Open(filepath.Join(m.stateHome, "romeu"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	v := cli.Version{Version: "v1.0.0", Commit: "abc", Protocol: 1}
	check := CheckFunc(st, &sbxdrv.Runner{Sbx: h.Tools.Sbx, Home: m.home, Host: &h}, v)

	err = check(t.Context())
	require.Error(t, err)
	assert.Equal(t, "RJ-302", cli.Of(err).ID, "no record")

	got := approveOn(t, m, terminal, master, "y\n")
	require.Equal(t, 0, got.code, got.stderr)
	require.NoError(t, check(t.Context()), "the acknowledged toolchain")

	other := CheckFunc(st, &sbxdrv.Runner{Sbx: h.Tools.Sbx, Home: m.home, Host: &h}, cli.Version{Version: "v1.1.0", Protocol: 1})
	err = other(t.Context())
	require.Error(t, err)
	assert.Equal(t, "RJ-302", cli.Of(err).ID, "another romeu version")
}

// mustRead returns the bytes of the file at path.
func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}
