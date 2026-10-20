// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package container

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCacheKeepsItsTemporaryFileAndModeToItself checks that the
// temporary file of a cached download is made in the cache directory and
// never in $TMPDIR, and that the file ends with mode 0755 whatever the
// umask. The cache never writes over a file that is there, so no file of
// another mode precedes it.
func TestCacheKeepsItsTemporaryFileAndModeToItself(t *testing.T) {
	srv := release(t, 200, []byte("binary"))
	c := fixtureConfig(t, srv.URL, []byte("binary"))
	tmpdir := t.TempDir()
	t.Setenv("TMPDIR", tmpdir)

	path, err := c.HerdrBinary(t.Context(), t.TempDir())
	require.NoError(t, err)
	require.FileExists(t, path)

	cached := filepath.Join(c.CacheDir, "linux-arm64", hexSum([]byte("binary")), "herdr")
	info, err := os.Stat(cached)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	for dir, want := range map[string][]string{tmpdir: {}, filepath.Dir(cached): {"herdr"}} {
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		got := []string{}
		for _, e := range entries {
			got = append(got, e.Name())
		}
		assert.Equal(t, want, got, "the entries of %s", dir)
	}
}
