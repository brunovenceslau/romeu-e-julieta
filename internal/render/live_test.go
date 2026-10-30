// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
)

func TestReadLive(t *testing.T) {
	t.Parallel()
	t.Run("an env dir that does not exist holds no record", func(t *testing.T) {
		t.Parallel()
		_, found, err := render.ReadLive(filepath.Join(t.TempDir(), "shop-env"))
		require.NoError(t, err)
		assert.False(t, found)
	})
	t.Run("an env dir before its first promotion holds no record", func(t *testing.T) {
		t.Parallel()
		_, found, err := render.ReadLive(t.TempDir())
		require.NoError(t, err)
		assert.False(t, found)
	})
	t.Run("a record reads back", func(t *testing.T) {
		t.Parallel()
		env := t.TempDir()
		want := shopRecord(t)
		data, err := render.EncodeRecord(want)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Join(env, ".romeu"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(env, render.LiveRecord), data, 0o600))
		got, found, err := render.ReadLive(env)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, want.SpecSource, got.SpecSource)
		assert.Equal(t, want.RecreateDigest, got.RecreateDigest)
	})
	t.Run("a record that does not decode is ErrRecordInvalid", func(t *testing.T) {
		t.Parallel()
		env := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(env, ".romeu"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(env, render.LiveRecord), []byte("{"), 0o600))
		_, _, err := render.ReadLive(env)
		require.ErrorIs(t, err, render.ErrRecordInvalid)
	})
}

// within fails the test unless fn returns within a few seconds: a read
// that waits on a FIFO for a writer never returns.
func within(t *testing.T, fn func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("the read blocked")
		return nil
	}
}

func TestReadLiveRefuses(t *testing.T) {
	t.Parallel()
	t.Run("a render.json that is a FIFO, without waiting for a writer", func(t *testing.T) {
		t.Parallel()
		env := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(env, ".romeu"), 0o700))
		require.NoError(t, syscall.Mkfifo(filepath.Join(env, render.LiveRecord), 0o600))
		err := within(t, func() error { _, _, err := render.ReadLive(env); return err })
		require.ErrorContains(t, err, "not a regular file")
	})
	t.Run("an env dir that is a file", func(t *testing.T) {
		t.Parallel()
		env := filepath.Join(t.TempDir(), "shop-env")
		require.NoError(t, os.WriteFile(env, nil, 0o600))
		_, _, err := render.ReadLive(env)
		require.Error(t, err)
	})
}

func TestDrift(t *testing.T) {
	t.Parallel()
	sum := sha256.Sum256([]byte("live\n"))
	rec := &render.Record{Files: map[string]string{
		render.LiveEnvFile:      hex.EncodeToString(sum[:]),
		".romeu/bin/SHA256SUMS": strings.Repeat("3", 64),
	}}
	tests := []struct {
		name  string
		setup func(t *testing.T, env string)
		rec   *render.Record
		want  []string
	}{
		{"an env dir that does not exist", func(*testing.T, string) {}, rec, nil},
		{"no render.json and no live sbxenv.yaml", mkdir, nil, nil},
		{"no render.json and a live sbxenv.yaml", liveEnv("live\n"), nil, []string{render.LiveEnvFile}},
		{"a matching file and a missing one", liveEnv("live\n"), rec, []string{".romeu/bin/SHA256SUMS"}},
		{"an edited file", liveEnv("edited\n"), rec, []string{".romeu/bin/SHA256SUMS", render.LiveEnvFile}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := filepath.Join(t.TempDir(), "shop-env")
			tt.setup(t, env)
			got, err := render.Drift(env, tt.rec)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
	t.Run("a live file that is a FIFO, without waiting for a writer", func(t *testing.T) {
		t.Parallel()
		env := t.TempDir()
		require.NoError(t, syscall.Mkfifo(filepath.Join(env, render.LiveEnvFile), 0o600))
		err := within(t, func() error { _, err := render.Drift(env, rec); return err })
		require.ErrorContains(t, err, "not a regular file")
	})
}

// mkdir makes the env dir.
func mkdir(t *testing.T, env string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(env, 0o700))
}

// liveEnv returns a setup that writes a live sbxenv.yaml of content.
func liveEnv(content string) func(t *testing.T, env string) {
	return func(t *testing.T, env string) {
		t.Helper()
		mkdir(t, env)
		require.NoError(t, os.WriteFile(filepath.Join(env, render.LiveEnvFile), []byte(content), 0o600))
	}
}
