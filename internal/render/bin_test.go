// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
)

func TestJulietaBinaryName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "julieta-linux-"+runtime.GOARCH, render.JulietaBinaryName(""))
	assert.Equal(t, "julieta-linux-arm64", render.JulietaBinaryName("arm64"))
	assert.Equal(t, "julieta-linux-amd64", render.JulietaBinaryName("amd64"))
}

func TestPrepareBinDetectsPlaceholder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  render.JulietaSource
	}{
		{"nil src", nil},
		{"default placeholder src", render.DefaultJulietaSource},
		{"empty bytes", func() ([]byte, error) { return []byte{}, nil }},
		{"nil slice", func() ([]byte, error) { return nil, nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := render.PrepareBin(tt.src, "arm64")
			require.Error(t, err)
			testsupport.RequireID(t, "RJ-312", err)
			var cliErr *cli.Error
			require.ErrorAs(t, err, &cliErr)
			assert.Equal(t, 2, cliErr.Exit)
			assert.Equal(t, "julieta-not-embedded", cliErr.Slug)
		})
	}
}

func TestPrepareBinSourceError(t *testing.T) {
	t.Parallel()
	boom := errors.New("cannot read binary")
	src := func() ([]byte, error) { return nil, boom }
	_, err := render.PrepareBin(src, "arm64")
	require.ErrorIs(t, err, boom)
}

func TestPrepareBinSuccess(t *testing.T) {
	t.Parallel()
	data := []byte("#!/bin/sh\necho julieta\n")
	src := func() ([]byte, error) { return data, nil }

	payload, err := render.PrepareBin(src, "arm64")
	require.NoError(t, err)

	assert.Equal(t, "julieta-linux-arm64", payload.BinaryName)
	assert.Equal(t, data, payload.BinaryData)

	wantBinSum := sha256.Sum256(data)
	wantBinHex := hex.EncodeToString(wantBinSum[:])
	assert.Equal(t, wantBinHex, payload.BinarySHA256)

	wantSumsData := wantBinHex + "  julieta-linux-arm64\n"
	assert.Equal(t, []byte(wantSumsData), payload.SumsData)

	wantSumsSum := sha256.Sum256([]byte(wantSumsData))
	wantSumsHex := hex.EncodeToString(wantSumsSum[:])
	assert.Equal(t, wantSumsHex, payload.SumsSHA256)

	files := payload.Files()
	assert.Equal(t, wantBinHex, files[".romeu/bin/julieta-linux-arm64"])
	assert.Equal(t, wantSumsHex, files[".romeu/bin/SHA256SUMS"])
}

func TestWriteBin(t *testing.T) {
	t.Parallel()
	envDir := t.TempDir()
	data := []byte("#!/bin/sh\necho julieta\n")
	src := func() ([]byte, error) { return data, nil }

	payload, err := render.WriteBin(envDir, src, "arm64")
	require.NoError(t, err)

	binPath := filepath.Join(envDir, ".romeu", "bin", "julieta-linux-arm64")
	gotBin, err := os.ReadFile(binPath)
	require.NoError(t, err)
	assert.Equal(t, data, gotBin)

	info, err := os.Stat(binPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())

	sumsPath := filepath.Join(envDir, ".romeu", "bin", "SHA256SUMS")
	gotSums, err := os.ReadFile(sumsPath)
	require.NoError(t, err)
	assert.Equal(t, payload.SumsData, gotSums)

	sumsInfo, err := os.Stat(sumsPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(render.FileMode), sumsInfo.Mode().Perm())
}

func TestWriteBinOpenRootFails(t *testing.T) {
	t.Parallel()
	data := []byte("#!/bin/sh\necho julieta\n")
	src := func() ([]byte, error) { return data, nil }
	// Pass a path that does not exist — os.OpenRoot must fail.
	_, err := render.WriteBin("/nonexistent/path/that/does/not/exist", src, "arm64")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open env dir")
}

func TestWriteBinPlaceholderRefused(t *testing.T) {
	t.Parallel()
	envDir := t.TempDir()
	_, err := render.WriteBin(envDir, nil, "arm64")
	require.Error(t, err)
	testsupport.RequireID(t, "RJ-312", err)

	// Asserts nothing was written.
	_, err = os.Stat(filepath.Join(envDir, ".romeu", "bin"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}
