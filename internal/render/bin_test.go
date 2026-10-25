// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
)

// fixtureJulieta is the fixture the embed seam is filled with: a few
// bytes, never a build.
var fixtureJulieta = []byte("#!fixture julieta\n")

func sumOf(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestNewBinRefusesWhatIsNotAnArchOrABinary(t *testing.T) {
	t.Parallel()
	_, err := render.NewBin("arm64", nil)
	require.ErrorIs(t, err, render.ErrJulietaNotEmbedded)
	for _, arch := range []string{"", "../x", "arm 64", "ARM64", "a/b"} {
		_, err := render.NewBin(arch, fixtureJulieta)
		require.ErrorIs(t, err, render.ErrBinArch, arch)
	}
}

func TestBinFilesRecordTheBinaryAndSumsDigests(t *testing.T) {
	t.Parallel()
	b, err := render.NewBin("arm64", fixtureJulieta)
	require.NoError(t, err)
	sums := sumOf(fixtureJulieta) + "  julieta-linux-arm64\n"
	assert.Equal(t, map[string]string{
		".romeu/bin/julieta-linux-arm64": sumOf(fixtureJulieta),
		".romeu/bin/SHA256SUMS":          sumOf([]byte(sums)),
	}, b.Files())
	assert.Equal(t, ".romeu/bin/julieta-linux-arm64", b.Path())
}

func TestBinFilesFitTheRecord(t *testing.T) {
	t.Parallel()
	b, err := render.NewBin("amd64", fixtureJulieta)
	require.NoError(t, err)
	r := shopRecord(t)
	r.Files = b.Files()
	data, err := render.EncodeRecord(r)
	require.NoError(t, err)
	got, err := render.DecodeRecord(data)
	require.NoError(t, err)
	assert.Equal(t, sumOf(fixtureJulieta), got.Files[b.Path()])
}

func TestWriteBinWritesTheBinaryAndSHA256SUMS(t *testing.T) {
	t.Parallel()
	env := t.TempDir()
	b, err := render.NewBin("arm64", fixtureJulieta)
	require.NoError(t, err)
	require.NoError(t, b.Write(env))

	bin := filepath.Join(env, ".romeu", "bin")
	got, err := os.ReadFile(filepath.Join(bin, "julieta-linux-arm64"))
	require.NoError(t, err)
	assert.Equal(t, fixtureJulieta, got)
	sums, err := os.ReadFile(filepath.Join(bin, "SHA256SUMS"))
	require.NoError(t, err)
	// The form sha256sum -c reads: digest, two spaces, bare name, newline.
	assert.Equal(t, sumOf(fixtureJulieta)+"  julieta-linux-arm64\n", string(sums))

	for name, mode := range map[string]os.FileMode{"julieta-linux-arm64": 0o755, "SHA256SUMS": 0o644} {
		fi, err := os.Stat(filepath.Join(bin, name))
		require.NoError(t, err)
		assert.Equal(t, mode, fi.Mode().Perm(), name)
	}
	entries, err := os.ReadDir(bin)
	require.NoError(t, err)
	assert.Len(t, entries, 2, "no temp file is left")
}

func TestWriteBinReplacesAnOlderBinary(t *testing.T) {
	t.Parallel()
	env := t.TempDir()
	old, err := render.NewBin("arm64", []byte("old"))
	require.NoError(t, err)
	require.NoError(t, old.Write(env))
	b, err := render.NewBin("arm64", fixtureJulieta)
	require.NoError(t, err)
	require.NoError(t, b.Write(env))
	require.NoError(t, b.Write(env), "a repeat is idempotent")

	got, err := os.ReadFile(filepath.Join(env, ".romeu", "bin", "julieta-linux-arm64"))
	require.NoError(t, err)
	assert.Equal(t, fixtureJulieta, got)
}

func TestWriteBinStaysInsideTheEnvDir(t *testing.T) {
	t.Parallel()
	env := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(env, ".romeu"), 0o700))
	require.NoError(t, os.Symlink(outside, filepath.Join(env, ".romeu", "bin")))
	b, err := render.NewBin("arm64", fixtureJulieta)
	require.NoError(t, err)
	require.Error(t, b.Write(env))
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
