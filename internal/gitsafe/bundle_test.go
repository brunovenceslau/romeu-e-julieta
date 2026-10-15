// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBundleCreate writes a bundle of named refs under the hardened
// flags, minus the objects reachable from the base, and plain git
// reads it back: bundle verify names the base as its prerequisite and
// lists the refs.
func TestBundleCreate(t *testing.T) {
	for _, mode := range []Mode{Host, Sandbox} {
		t.Run(mode.String(), func(t *testing.T) {
			dir, base := newRepo(t)
			head := commit(t, dir, "two")
			plainGit(t, dir, "branch", "topic")
			file := filepath.Join(t.TempDir(), "heads.bundle")
			r := newRunner(t, mode)
			require.NoError(t, r.BundleCreate(t.Context(), dir, file, []string{"refs/heads/main", "refs/heads/topic"}, []string{base}))

			out := plainGit(t, dir, "bundle", "verify", file)
			assert.Contains(t, out, base, "the base is a prerequisite")
			heads := plainGit(t, dir, "bundle", "list-heads", file)
			assert.Equal(t, head+" refs/heads/main\n"+head+" refs/heads/topic", heads)

			full := filepath.Join(t.TempDir(), "full.bundle")
			require.NoError(t, r.BundleCreate(t.Context(), dir, full, []string{"refs/heads/main"}, nil))
			fresh := t.TempDir()
			plainGit(t, fresh, "init", "-q")
			plainGit(t, fresh, "bundle", "verify", full)
			_, err := plainGitErr(t, fresh, nil, "bundle", "verify", file)
			require.Error(t, err, "the thin bundle needs its base")
		})
	}
}

// TestBundleCreateRefuses: only an absolute file, full ref names and
// full object ids reach the argv.
func TestBundleCreateRefuses(t *testing.T) {
	oid := strings.Repeat("d", 40)
	tests := []struct {
		name  string
		file  string
		refs  []string
		bases []string
	}{
		{"relative file", "heads.bundle", []string{"refs/heads/main"}, nil},
		{"option file", "-", []string{"refs/heads/main"}, nil},
		{"no ref", "/b", nil, nil},
		{"short ref", "/b", []string{"main"}, nil},
		{"option ref", "/b", []string{"--all"}, nil},
		{"pattern ref", "/b", []string{"refs/heads/*"}, nil},
		{"short base", "/b", []string{"refs/heads/main"}, []string{"main"}},
		{"option base", "/b", []string{"refs/heads/main"}, []string{"--all"}},
		{"bad base", "/b", []string{"refs/heads/main"}, []string{oid + "x"}},
	}
	r, err := New(Config{Mode: Sandbox, Git: "/usr/bin/git", Home: "/h"})
	require.NoError(t, err)
	var rec recorder
	rec.install(r)
	for _, tt := range tests {
		require.ErrorIs(t, r.BundleCreate(t.Context(), "/repo", tt.file, tt.refs, tt.bases), ErrRefused, tt.name)
	}
	assert.Empty(t, rec.commands)

	require.NoError(t, r.BundleCreate(t.Context(), "/repo", "/b", []string{"refs/heads/main"}, []string{oid}))
	require.Len(t, rec.commands, 1)
	assert.Equal(t, []string{"bundle", "create", "/b", "refs/heads/main", "^" + oid}, rec.commands[0].Argv[1+len(sandboxFlags):])
}
