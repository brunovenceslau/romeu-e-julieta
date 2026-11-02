// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package upgrade

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// TestRunningBinary holds REQ-1326 and REQ-1335: the path of the
// running binary with its links resolved, and binary-unwritable when
// the user cannot write its directory, when its group or others may
// write it, or when another user owns it, each with a message that
// names the mode. It replaces the seams of the package, so it does not
// run in parallel.
func TestRunningBinary(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	real := filepath.Join(dir, "real", "romeu")
	require.NoError(t, os.Mkdir(filepath.Dir(real), 0o700))
	require.NoError(t, os.WriteFile(real, []byte(oldBinary), 0o755))
	link := filepath.Join(dir, "romeu")
	require.NoError(t, os.Symlink(real, link))
	oldExe, oldWritable := executable, writable
	t.Cleanup(func() { executable, writable = oldExe, oldWritable })
	executable = func() (string, error) { return link, nil }

	got, err := runningBinary()
	require.NoError(t, err)
	assert.Equal(t, real, got, "the links are resolved")

	var asked string
	writable = func(d string) error { asked = d; return errors.New("permission denied") }
	_, err = runningBinary()
	requireID(t, err, BinaryUnwritable)
	assert.Equal(t, filepath.Dir(real), asked, "the directory judged is the one of the resolved path")
	assert.Contains(t, cli.Of(err).Details, cli.Detail{Key: "directory", Value: filepath.Dir(real)})

	writable = oldWritable
	for mode, shown := range map[os.FileMode]string{0o770: "drwxrwx---", 0o702: "drwx----w-", os.ModeSticky | 0o777: "dtrwxrwxrwx"} {
		require.NoError(t, os.Chmod(filepath.Dir(real), mode))
		_, err = runningBinary()
		requireID(t, err, BinaryUnwritable)
		require.ErrorContains(t, err, "writable by its group or others (mode "+shown+")")
	}
	require.NoError(t, os.Chmod(filepath.Dir(real), 0o500))
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(real), 0o700) })
	if os.Geteuid() != 0 {
		_, err = runningBinary()
		requireID(t, err, BinaryUnwritable)

		// A directory another user owns: /usr/bin, root's.
		executable = func() (string, error) { return "/usr/bin/true", nil }
		_, err = runningBinary()
		requireID(t, err, BinaryUnwritable)
		assert.ErrorContains(t, err, "not owned by this user (mode d")
	}
}

// TestExtract holds REQ-1328: the romeu member of the archive is
// written under a temporary name beside the running binary, mode 0755;
// an archive without it, one whose member is no regular file, and one
// that is not a whole tar.gz are verify-failed, and leave no file.
func TestExtract(t *testing.T) {
	t.Parallel()
	link := func() []byte {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		require.NoError(t, tw.WriteHeader(&tar.Header{Typeflag: tar.TypeSymlink, Name: binaryMember, Linkname: "/bin/sh"}))
		require.NoError(t, tw.Close())
		require.NoError(t, gz.Close())
		return buf.Bytes()
	}
	whole := archiveOf(t, [2]string{"COPYING", "license"}, [2]string{binaryMember, newBinaryScript})

	t.Run("the romeu member", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		p, sum, err := extract(whole, dir)
		require.NoError(t, err)
		assert.Equal(t, sha256.Sum256([]byte(newBinaryScript)), sum, "the sha256 of the bytes written")
		assert.Equal(t, dir, filepath.Dir(p))
		assert.True(t, strings.HasPrefix(filepath.Base(p), ".romeu-upgrade-"), p)
		data, err := os.ReadFile(p)
		require.NoError(t, err)
		assert.Equal(t, newBinaryScript, string(data))
		info, err := os.Stat(p)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	})
	for name, archive := range map[string][]byte{
		"no romeu member":        archiveOf(t, [2]string{"COPYING", "license"}),
		"a link named romeu":     link(),
		"not gzip":               []byte("plain"),
		"cut inside the archive": whole[:len(whole)/2],
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			_, _, err := extract(archive, dir)
			requireID(t, err, VerifyFailed)
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			assert.Empty(t, entries, "nothing is left beside the running binary")
		})
	}
}

// TestSwap holds the swap step (REQ-1330): the new binary is checked,
// renamed over the running one, the finish token is written, and the
// process is replaced by the new binary, as romeu upgrade --finish
// <running version>, with the process's own environment. The swap runs
// in this test binary run again as a helper process
// (TestHelperProcess), whose process it replaces.
func TestSwap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	state := t.TempDir()
	exe := filepath.Join(dir, "romeu")
	require.NoError(t, os.WriteFile(exe, []byte(oldBinary), 0o755))
	temp := filepath.Join(dir, ".romeu-upgrade-1")
	script := "#!/bin/sh\necho \"swapped $* $UPGRADE_TEST_MARK\"\n"
	require.NoError(t, os.WriteFile(temp, []byte(script), 0o755))
	require.NoError(t, os.Chmod(temp, 0o755))
	sum := sha256.Sum256([]byte(script))
	self, err := os.Executable()
	require.NoError(t, err)
	var out bytes.Buffer
	err = procgroup.Run(t.Context(), procgroup.Command{
		Class: procgroup.ToolStep,
		Argv:  []string{self, "-test.run=^TestHelperProcess$"},
		Env: []string{helperEnv + "=swap", "UPGRADE_TEST_SWAP_TEMP=" + temp, "UPGRADE_TEST_SWAP_EXE=" + exe,
			"UPGRADE_TEST_SWAP_STATE=" + state, "UPGRADE_TEST_SWAP_SUM=" + hex.EncodeToString(sum[:]), "UPGRADE_TEST_MARK=kept"},
		Stdout: &out, Stderr: &out,
	})
	require.NoError(t, err, out.String())
	assert.Equal(t, "swapped upgrade --finish v1.0.0-alpha.1 kept\n", out.String())
	data, err := os.ReadFile(exe)
	require.NoError(t, err)
	assert.Equal(t, script, string(data))
	assert.NoFileExists(t, temp)
	tok, err := consumeToken(state)
	require.NoError(t, err, "the swap left the finish token")
	assert.Equal(t, finishToken{version: "v1.0.0-alpha.1", sum: sum, expires: time.Date(2026, 11, 2, 9, 1, 0, 0, time.UTC)}, tok)
}

// TestSwapRefuses: a new binary that changed after its extraction (its
// content, its mode, a link in its place) is verify-failed and removed,
// and the running binary is as it was; a rename that fails leaves the
// running binary as it was; an exec that fails returns and leaves no
// finish token.
func TestSwapRefuses(t *testing.T) {
	t.Parallel()
	script := "#!/bin/sh\necho new\n"
	for _, tt := range []struct {
		name    string
		arrange func(t *testing.T, temp string)
	}{
		{"another content", func(t *testing.T, temp string) {
			t.Helper()
			require.NoError(t, os.WriteFile(temp, []byte(script+"echo planted\n"), 0o755))
		}},
		{"another mode", func(t *testing.T, temp string) {
			t.Helper()
			require.NoError(t, os.WriteFile(temp, []byte(script), 0o755))
			require.NoError(t, os.Chmod(temp, 0o775))
		}},
		{"a link in its place", func(t *testing.T, temp string) {
			t.Helper()
			target := filepath.Join(t.TempDir(), "target")
			require.NoError(t, os.WriteFile(target, []byte(script), 0o755))
			require.NoError(t, os.Symlink(target, temp))
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir, state := t.TempDir(), t.TempDir()
			exe := filepath.Join(dir, "romeu")
			require.NoError(t, os.WriteFile(exe, []byte(oldBinary), 0o755))
			temp := filepath.Join(dir, ".romeu-upgrade-1")
			tt.arrange(t, temp)
			err := swap(swapStep{temp: temp, exe: exe, sum: sha256.Sum256([]byte(script)), stateDir: state, running: "v1.0.0"})
			requireID(t, err, VerifyFailed)
			_, err = os.Lstat(temp)
			require.ErrorIs(t, err, fs.ErrNotExist, "the new binary is removed")
			data, err := os.ReadFile(exe)
			require.NoError(t, err)
			assert.Equal(t, oldBinary, string(data))
			assert.NoFileExists(t, filepath.Join(state, tokenName))
		})
	}
	t.Run("a rename that fails", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		exe := filepath.Join(dir, "romeu")
		temp := filepath.Join(dir, ".romeu-upgrade-1")
		require.NoError(t, os.WriteFile(temp, []byte(script), 0o755))
		require.NoError(t, os.Mkdir(exe, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(exe, "in"), nil, 0o600))
		requireID(t, swap(swapStep{temp: temp, exe: exe, sum: sha256.Sum256([]byte(script)), stateDir: t.TempDir()}), cli.Internal)
		assert.DirExists(t, exe)
	})
	t.Run("an exec that fails", func(t *testing.T) {
		t.Parallel()
		dir, state := t.TempDir(), t.TempDir()
		exe := filepath.Join(dir, "romeu")
		temp := filepath.Join(dir, ".romeu-upgrade-2")
		require.NoError(t, os.WriteFile(temp, []byte("not a program"), 0o755))
		require.NoError(t, os.Chmod(temp, 0o755))
		err := swap(swapStep{temp: temp, exe: exe, sum: sha256.Sum256([]byte("not a program")), stateDir: state, running: "v1.0.0",
			expires: time.Now().Add(time.Minute)})
		requireID(t, err, cli.Internal)
		require.ErrorContains(t, err, "did not start")
		assert.NoFileExists(t, filepath.Join(state, tokenName), "no token outlives a failed exec")
	})
}
