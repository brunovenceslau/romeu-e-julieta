// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDoctorMemoryLayout: a memory dir holding what the layout
// allowlist of 08 8.1 does not admit, or an entry of memory/ that is
// not a directory, fails memory-layout (REQ-0387).
func TestDoctorMemoryLayout(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, want string
		plant      func(t *testing.T, memory string)
	}{
		{"a file the allowlist does not admit", "shop-api/notes.txt", func(t *testing.T, memory string) {
			t.Helper()
			require.NoError(t, os.WriteFile(filepath.Join(memory, "shop-api", "notes.txt"), []byte("x"), 0o600))
		}},
		{"a link in place of a memory dir", "memory/elsewhere", func(t *testing.T, memory string) {
			t.Helper()
			require.NoError(t, os.Symlink("/", filepath.Join(memory, "elsewhere")))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			env := filepath.Join(f.root, "shop-env")
			memory := filepath.Join(env, "memory")
			require.NoError(t, os.MkdirAll(filepath.Join(memory, "shop-api", "entries"), 0o700))
			require.NoError(t, os.MkdirAll(filepath.Join(memory, "_shared", "entries"), 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(env, neverIndex), nil, 0o600))

			rep, out, err := f.output(t)
			require.NoError(t, err, out)
			assert.Equal(t, resultPass, check(t, rep, "memory-layout").Result, "a memory dir in its layout")

			tc.plant(t, memory)
			rep, out, err = f.output(t)

			c := check(t, rep, "memory-layout")
			assert.Equal(t, resultFail, c.Result, out)
			assert.Contains(t, c.Message, tc.want)
			assert.Equal(t, 6, exitOf(err))
		})
	}
}

// TestDoctorMemoryLayoutReportsALink: a link in place of a memory/ of
// the tree fails memory-layout and is not followed.
func TestDoctorMemoryLayoutReportsALink(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	env := filepath.Join(f.root, "shop-env")
	require.NoError(t, os.Mkdir(env, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(env, neverIndex), nil, 0o600))
	target := filepath.Join(f.tmp, "elsewhere")
	require.NoError(t, os.MkdirAll(filepath.Join(target, "shop-api", "stray"), 0o700))
	require.NoError(t, os.Symlink(target, filepath.Join(env, "memory")))

	rep, out, _ := f.output(t)

	c := check(t, rep, "memory-layout")
	assert.Equal(t, resultFail, c.Result, out)
	assert.Contains(t, c.Message, "memory is a link, not the directory romeu made")
	assert.NotContains(t, c.Message, "stray", "the link is not followed")
}
