// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render_test

import (
	"os"
	"path/filepath"
	"testing"

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
