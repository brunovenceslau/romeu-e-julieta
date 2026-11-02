// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package upgrade

import (
	"crypto/sha256"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/approve"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// syncRuns records the runs of the command sync of this test binary,
// and syncResult is what it returns. The tests that read them do not
// run in parallel.
var (
	syncRuns   [][]string
	syncResult error
)

// The command sync of this test binary alone: it stands for romeu sync,
// which this package does not link, and records its flags.
var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightRuns, cli.LockTaken, cli.AppendsNothing(), cli.SnapshotNone, cli.Converges)
	c.Binary, c.Name = "romeu", "sync"
	c.NoJourney = "a fixture of the tests of internal/cmd/romeu/upgrade"
	c.New = func(*cli.Env) *cobra.Command {
		var all bool
		cmd := &cobra.Command{
			Use:   "sync [--all]",
			Short: "a command of the tests",
			Args:  cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				if all {
					syncRuns = append(syncRuns, []string{"--all"})
				} else {
					syncRuns = append(syncRuns, nil)
				}
				return syncResult
			},
		}
		cmd.Flags().BoolVar(&all, "all", false, "every project")
		return cmd
	}
	return c
}())

// swapTime is the clock of runCommand, and expiry the expiry of a
// finish token a swap wrote at that time.
var (
	swapTime = time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	expiry   = swapTime.Add(tokenLife)
)

// TestFinish holds REQ-1330 and REQ-0061: romeu upgrade --finish
// <running version>, the process the swap step starts on the terminal
// of the command, writes toolchain.json with no prompt, from the binary
// it runs as, then runs the steps of romeu sync --all, whose result is
// the command's; the finish token of the swap is consumed.
func TestFinish(t *testing.T) {
	m := newMachine(t)
	_, terminal := openPTY(t)
	_ = use(t, m, terminal, int(terminal.Fd()), nil)
	syncRuns, syncResult = nil, nil
	t.Cleanup(func() { syncRuns, syncResult = nil, nil })
	m.writeToken(t, "v1.0.0-alpha.0", oldBinary, expiry)

	got := runCommand(t, m, nil, "", "upgrade", "--finish", "v1.0.0-alpha.0")

	require.Equal(t, 0, got.code, got.stderr)
	assert.Equal(t, "upgraded      romeu v1.0.0-alpha.0 -> v1.0.0-alpha.1\nacknowledged  "+
		filepath.Join(m.stateHome, "romeu", "toolchain.json")+"\n", got.stdout)
	assert.Equal(t, [][]string{{"--all"}}, syncRuns)
	st, err := state.OpenRead(filepath.Join(m.stateHome, "romeu"))
	require.NoError(t, err)
	defer func() { _ = st.Close() }()
	rec, err := approve.Load(st)
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, "v1.0.0-alpha.1", rec.Romeu.Version)
	assert.Equal(t, "0.49.2", rec.SbxVersion)
	assert.NoFileExists(t, filepath.Join(m.stateHome, "romeu", tokenName), "the token is consumed")

	m.writeToken(t, "v1.0.0-alpha.0", oldBinary, expiry)
	syncResult = cli.Usage.New("sync refused")
	got = runCommand(t, m, nil, "", "upgrade", "--finish", "v1.0.0-alpha.0")
	assert.Equal(t, 2, got.code, "the result of sync is the command's")
}

// TestFinishStopsAtTheLock: a finish that cannot take the state lock
// writes nothing and runs no sync.
func TestFinishStopsAtTheLock(t *testing.T) {
	m := newMachine(t)
	_, terminal := openPTY(t)
	_ = use(t, m, terminal, int(terminal.Fd()), nil)
	m.writeToken(t, "v1.0.0-alpha.0", oldBinary, expiry)
	syncRuns = nil
	t.Cleanup(func() { syncRuns = nil })
	st, err := state.Open(filepath.Join(m.stateHome, "romeu"))
	require.NoError(t, err)
	lk, err := st.Lock()
	require.NoError(t, err)
	t.Cleanup(func() { lk.Unlock(); _ = st.Close() })

	got := runCommand(t, m, nil, "", "upgrade", "--finish", "v1.0.0-alpha.0")

	assert.Equal(t, state.Locked.Exit, got.code, got.stderr)
	assert.Contains(t, got.stderr, "error "+state.Locked.ID+" ")
	assert.Empty(t, syncRuns)
	assert.NoFileExists(t, filepath.Join(m.stateHome, "romeu", "toolchain.json"))
}

// TestFinishRefuses holds the binding of --finish to a swap that just
// happened (REQ-1330): without a terminal on its standard input or its
// standard error, without the finish token of a swap, or with one that
// expired, that names another binary or another version, or that is
// not a regular file of this user in the line format, romeu upgrade
// --finish is the error usage (exit 2), writes no toolchain.json and
// runs no sync; and a token serves one finish.
func TestFinishRefuses(t *testing.T) {
	pipeRead, pipeWrite, err := os.Pipe()
	require.NoError(t, err)
	closeAtEnd(t, pipeRead)
	closeAtEnd(t, pipeWrite)
	for _, tt := range []struct {
		name    string
		arrange func(t *testing.T, m machine) (in io.Reader, errFD int)
		why     string
	}{
		{"a pipe on stdin", func(t *testing.T, m machine) (io.Reader, int) {
			t.Helper()
			m.writeToken(t, "v1.0.0-alpha.0", oldBinary, expiry)
			_, terminal := openPTY(t)
			return pipeRead, int(terminal.Fd())
		}, "needs a terminal on its standard input and its standard error"},
		{"a pipe on stderr", func(t *testing.T, m machine) (io.Reader, int) {
			t.Helper()
			m.writeToken(t, "v1.0.0-alpha.0", oldBinary, expiry)
			_, terminal := openPTY(t)
			return terminal, int(pipeWrite.Fd())
		}, "needs a terminal on its standard input and its standard error"},
		{"no token", func(t *testing.T, _ machine) (io.Reader, int) {
			t.Helper()
			_, terminal := openPTY(t)
			return terminal, int(terminal.Fd())
		}, "no swap left a token"},
		{"an expired token", func(t *testing.T, m machine) (io.Reader, int) {
			t.Helper()
			m.writeToken(t, "v1.0.0-alpha.0", oldBinary, swapTime)
			_, terminal := openPTY(t)
			return terminal, int(terminal.Fd())
		}, "expired at 2026-11-02T09:00:00Z"},
		{"a token further ahead than a swap writes it", func(t *testing.T, m machine) (io.Reader, int) {
			t.Helper()
			m.writeToken(t, "v1.0.0-alpha.0", oldBinary, expiry.Add(time.Second))
			_, terminal := openPTY(t)
			return terminal, int(terminal.Fd())
		}, "or is not of this swap"},
		{"a token of another binary", func(t *testing.T, m machine) (io.Reader, int) {
			t.Helper()
			m.writeToken(t, "v1.0.0-alpha.0", newBinaryScript, expiry)
			_, terminal := openPTY(t)
			return terminal, int(terminal.Fd())
		}, "names another binary than the running one"},
		{"a token of another version", func(t *testing.T, m machine) (io.Reader, int) {
			t.Helper()
			m.writeToken(t, "v1.0.0-alpha.9", oldBinary, expiry)
			_, terminal := openPTY(t)
			return terminal, int(terminal.Fd())
		}, "names the version v1.0.0-alpha.9, not v1.0.0-alpha.0"},
		{"a link in place of the token", func(t *testing.T, m machine) (io.Reader, int) {
			t.Helper()
			target := filepath.Join(m.home, "token")
			require.NoError(t, os.WriteFile(target, finishToken{version: "v1.0.0-alpha.0", sum: sha256.Sum256([]byte(oldBinary)), expires: expiry}.encode(), 0o600))
			require.NoError(t, os.Symlink(target, filepath.Join(m.stateHome, "romeu", tokenName)))
			_, terminal := openPTY(t)
			return terminal, int(terminal.Fd())
		}, "the token does not open"},
		{"a token others can read", func(t *testing.T, m machine) (io.Reader, int) {
			t.Helper()
			m.writeToken(t, "v1.0.0-alpha.0", oldBinary, expiry)
			require.NoError(t, os.Chmod(filepath.Join(m.stateHome, "romeu", tokenName), 0o644))
			_, terminal := openPTY(t)
			return terminal, int(terminal.Fd())
		}, "not a regular file of this user, mode 0600 (mode -rw-r--r--)"},
		{"a token of another format", func(t *testing.T, m machine) (io.Reader, int) {
			t.Helper()
			require.NoError(t, os.WriteFile(filepath.Join(m.stateHome, "romeu", tokenName), []byte("version v1.0.0-alpha.0\n"), 0o600))
			_, terminal := openPTY(t)
			return terminal, int(terminal.Fd())
		}, "not of the format romeu-upgrade-finish/1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newMachine(t)
			in, errFD := tt.arrange(t, m)
			_ = use(t, m, in, errFD, nil)
			syncRuns = nil
			t.Cleanup(func() { syncRuns = nil })

			got := runCommand(t, m, nil, "", "upgrade", "--finish", "v1.0.0-alpha.0")

			assert.Equal(t, 2, got.code, got.stderr)
			assert.Contains(t, got.stderr, "error RJ-349 usage: ")
			assert.Contains(t, got.stderr, tt.why)
			assert.Empty(t, got.stdout)
			assert.Empty(t, syncRuns, "no sync")
			assert.NoFileExists(t, filepath.Join(m.stateHome, "romeu", "toolchain.json"))
			if !strings.Contains(tt.why, "terminal") {
				_, err := os.Lstat(filepath.Join(m.stateHome, "romeu", tokenName))
				assert.ErrorIs(t, err, fs.ErrNotExist, "a token is consumed by the finish that reads it")
			}
		})
	}

	t.Run("a token serves one finish", func(t *testing.T) {
		m := newMachine(t)
		_, terminal := openPTY(t)
		_ = use(t, m, terminal, int(terminal.Fd()), nil)
		syncRuns = nil
		t.Cleanup(func() { syncRuns = nil })
		m.writeToken(t, "v1.0.0-alpha.0", oldBinary, expiry)

		got := runCommand(t, m, nil, "", "upgrade", "--finish", "v1.0.0-alpha.0")
		require.Equal(t, 0, got.code, got.stderr)
		require.NoError(t, os.Remove(filepath.Join(m.stateHome, "romeu", "toolchain.json")))

		got = runCommand(t, m, nil, "", "upgrade", "--finish", "v1.0.0-alpha.0")
		assert.Equal(t, 2, got.code, got.stderr)
		assert.Contains(t, got.stderr, "no swap left a token")
		assert.NoFileExists(t, filepath.Join(m.stateHome, "romeu", "toolchain.json"))
		assert.Len(t, syncRuns, 1, "one sync, from the first finish")
	})
}

// TestToken holds the line format of the finish token: what encode
// writes parseToken reads back, and parseToken refuses anything else.
func TestToken(t *testing.T) {
	t.Parallel()
	tok := finishToken{version: "v0.0.0-dev+0123456789abcdef-dirty", sum: sha256.Sum256([]byte("x")), expires: expiry}
	data := tok.encode()
	assert.Equal(t, "romeu-upgrade-finish/1\nversion v0.0.0-dev+0123456789abcdef-dirty\nsha256 "+
		"2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881\nexpires 2026-11-02T09:01:00Z\n", string(data))
	got, err := parseToken(data)
	require.NoError(t, err)
	assert.Equal(t, tok, got)
	for _, bad := range []string{
		"",
		strings.TrimSuffix(string(data), "\n"),
		string(data) + "more\n",
		strings.Replace(string(data), "romeu-upgrade-finish/1", "romeu-upgrade-finish/2", 1),
		strings.Replace(string(data), "version v0", "version latest", 1),
		strings.Replace(string(data), "sha256 2d", "sha256 2D", 1),
		strings.Replace(string(data), "sha256 2d", "sha256 ", 1),
		strings.Replace(string(data), "2026-11-02T09:01:00Z", "tomorrow", 1),
	} {
		_, err := parseToken([]byte(bad))
		assert.ErrorIs(t, err, errTokenFormat, "%q", bad)
	}
}
