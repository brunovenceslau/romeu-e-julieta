// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
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

// TestCommitFileAtTheLimit: a file of exactly limit bytes is read, and
// one of limit+1 bytes is refused (TestCommitFileRefuses).
func TestCommitFileAtTheLimit(t *testing.T) {
	dir, _ := newRepo(t)
	exact := writeBlob(t, dir, strings.Repeat("x", 10))
	c := commitTree(t, dir, writeTree(t, dir, rawEntry{"100644", "exact", exact}))
	data, found, err := newRunner(t, Host).CommitFile(t.Context(), dir, c, "exact", 10)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, strings.Repeat("x", 10), string(data))
}

// TestCommitFileOverTheLimitReadsNoBlob: the size ls-tree reports refuses a
// file of limit+1 bytes, before git cat-file reads it.
func TestCommitFileOverTheLimitReadsNoBlob(t *testing.T) {
	dir, _ := newRepo(t)
	big := writeBlob(t, dir, strings.Repeat("x", 11))
	c := commitTree(t, dir, writeTree(t, dir, rawEntry{"100644", "big", big}))
	r := newRunner(t, Host)
	var started [][]string
	run := r.start
	r.start = func(ctx context.Context, cmd procgroup.Command) error {
		started = append(started, cmd.Argv)
		return run(ctx, cmd)
	}
	_, _, err := r.CommitFile(t.Context(), dir, c, "big", 10)
	require.ErrorIs(t, err, ErrRefused)
	require.NotEmpty(t, started)
	for _, argv := range started {
		assert.NotContains(t, argv, "cat-file")
	}
}

// TestCommitFileRefuses: a path, a commit, a mode and a size gitsafe
// does not accept are each ErrRefused, and git never reads the blob of
// a file over the limit.
func TestCommitFileRefuses(t *testing.T) {
	dir, head := newRepo(t)
	blob := writeBlob(t, dir, "x\n")
	big := writeBlob(t, dir, strings.Repeat("x", 11)) // limit + 1
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
		file               bool // the refusal is of the file, ErrFileRefused
	}{
		{"a branch name", "HEAD", "big", "not a full object id", false},
		{"an empty path", c, "", "not a literal path", false},
		{"a pathspec", c, ":(top)big", "not a literal path", false},
		{"a glob", c, "b*", "not a literal path", false},
		{"a parent path", c, "../big", "not a clean relative path", false},
		{"a negative limit", c, "big", "negative limit", false},
		{"a file over the limit", c, "big", "more than 10 bytes", true},
		{"a symbolic link", c, "link", "not a regular file", true},
		{"a gitlink", c, "module", "not a regular file", true},
		{"a directory", c, "dir", "not a regular file", true},
	}
	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.name, " ", "_"), func(t *testing.T) {
			limit := 10
			if tt.name == "a negative limit" {
				limit = -1
			}
			_, _, err := r.CommitFile(t.Context(), dir, tt.commit, tt.path, limit)
			require.ErrorIs(t, err, ErrRefused)
			assert.Contains(t, err.Error(), tt.want)
			assert.Equal(t, tt.file, errors.Is(err, ErrFileRefused))
		})
	}
}
