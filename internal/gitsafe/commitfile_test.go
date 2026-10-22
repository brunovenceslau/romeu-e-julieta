// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommitFileRealGit reads a file of a commit by its path from the
// root, and says when the commit holds none.
func TestCommitFileRealGit(t *testing.T) {
	for _, mode := range []Mode{Host, Sandbox} {
		t.Run(mode.String(), func(t *testing.T) {
			dir, _ := newRepo(t)
			lock := writeBlob(t, dir, "lockfile_version = 3\n")
			empty := writeBlob(t, dir, "")
			nested := writeTree(t, dir, rawEntry{"100644", "lock-exceptions", lock})
			c := commitTree(t, dir, writeTree(t, dir,
				rawEntry{"40000", ".julieta", nested},
				rawEntry{"100644", "empty", empty},
				rawEntry{"100644", "mise.lock", lock},
			))
			r := newRunner(t, mode)

			data, found, err := r.CommitFile(t.Context(), dir, c, "mise.lock", 1<<20)
			require.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, "lockfile_version = 3\n", string(data))

			data, found, err = r.CommitFile(t.Context(), dir, c, ".julieta/lock-exceptions", 1<<20)
			require.NoError(t, err)
			assert.True(t, found)
			assert.Equal(t, "lockfile_version = 3\n", string(data))

			data, found, err = r.CommitFile(t.Context(), dir, c, "empty", 1<<20)
			require.NoError(t, err)
			assert.True(t, found)
			assert.Empty(t, data)

			for _, name := range []string{"mise.toml", "mise.loc", ".julieta/missing", "mise.lock/x"} {
				data, found, err = r.CommitFile(t.Context(), dir, c, name, 1<<20)
				require.NoError(t, err, name)
				assert.False(t, found, name)
				assert.Nil(t, data, name)
			}
		})
	}
}

// TestCommitFileRefuses: a path, a commit, a mode and a size gitsafe
// does not accept are each ErrRefused, and git never reads the blob of
// a file over the limit.
func TestCommitFileRefuses(t *testing.T) {
	dir, head := newRepo(t)
	blob := writeBlob(t, dir, "x\n")
	big := writeBlob(t, dir, strings.Repeat("x", 11))
	nested := writeTree(t, dir, rawEntry{"100644", "f", blob})
	c := commitTree(t, dir, writeTree(t, dir,
		rawEntry{"100644", "big", big},
		rawEntry{"40000", "dir", nested},
		rawEntry{"120000", "link", blob},
		rawEntry{"160000", "module", head},
	))
	r := newRunner(t, Host)
	tests := []struct {
		name, commit, path string
		want               string
	}{
		{"a branch name", "HEAD", "big", "not a full object id"},
		{"an empty path", c, "", "not a literal path"},
		{"a pathspec", c, ":(top)big", "not a literal path"},
		{"a glob", c, "b*", "not a literal path"},
		{"a parent path", c, "../big", "not a clean relative path"},
		{"a file over the limit", c, "big", "more than 10 bytes"},
		{"a symbolic link", c, "link", "not a regular file"},
		{"a gitlink", c, "module", "not a regular file"},
		{"a directory", c, "dir", "not a regular file"},
	}
	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.name, " ", "_"), func(t *testing.T) {
			_, _, err := r.CommitFile(t.Context(), dir, tt.commit, tt.path, 10)
			require.ErrorIs(t, err, ErrRefused)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}
