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

// TestBundleVerifyAndUnbundle verifies a bundle and writes its objects
// into a fresh repository, under the flags of each mode: Unbundle
// returns the refs and creates none, the objects are there after it,
// and a thin bundle whose base the repository lacks fails both.
func TestBundleVerifyAndUnbundle(t *testing.T) {
	for _, mode := range []Mode{Host, Sandbox} {
		t.Run(mode.String(), func(t *testing.T) {
			src, base := newRepo(t)
			head := commit(t, src, "two")
			plainGit(t, src, "branch", "topic")
			full := filepath.Join(t.TempDir(), "full.bundle")
			plainGit(t, src, "bundle", "create", full, "refs/heads/main", "refs/heads/topic")
			thin := filepath.Join(t.TempDir(), "thin.bundle")
			plainGit(t, src, "bundle", "create", thin, "refs/heads/main", "^"+base)

			dst := t.TempDir()
			plainGit(t, dst, "init", "-q")
			r := newRunner(t, mode)
			require.Error(t, r.BundleVerify(t.Context(), dst, thin), "the base is missing")
			_, err := r.Unbundle(t.Context(), dst, thin)
			require.Error(t, err)

			require.NoError(t, r.BundleVerify(t.Context(), dst, full))
			heads, err := r.Unbundle(t.Context(), dst, full)
			require.NoError(t, err)
			assert.Equal(t, []Head{{"refs/heads/main", head}, {"refs/heads/topic", head}}, heads)
			plainGit(t, dst, "cat-file", "-e", head+"^{commit}")
			assert.Empty(t, plainGit(t, dst, "for-each-ref"), "no ref is created")

			require.NoError(t, r.BundleVerify(t.Context(), dst, thin), "the base is there now")
		})
	}
}

// TestBundleVerifyAndUnbundleRefuse: only an absolute bundle file and a
// repository root reach the argv, which carries the flags of the mode.
func TestBundleVerifyAndUnbundleRefuse(t *testing.T) {
	r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: "/h"})
	require.NoError(t, err)
	var rec recorder
	rec.install(r)
	for _, file := range []string{"heads.bundle", "-", "--all"} {
		require.ErrorIs(t, r.BundleVerify(t.Context(), stubRepo, file), ErrRefused, file)
		_, err := r.Unbundle(t.Context(), stubRepo, file)
		require.ErrorIs(t, err, ErrRefused, file)
	}
	require.ErrorIs(t, r.BundleVerify(t.Context(), "relative", "/b"), ErrRefused)
	_, err = r.Unbundle(t.Context(), t.TempDir(), "/b")
	require.ErrorIs(t, err, ErrRefused, "a directory that is no repository root")
	assert.Empty(t, rec.commands)

	require.NoError(t, r.BundleVerify(t.Context(), stubRepo, "/b"))
	_, err = r.Unbundle(t.Context(), stubRepo, "/b")
	require.NoError(t, err)
	require.Len(t, rec.commands, 2)
	assert.Equal(t, []string{"bundle", "verify", "--quiet", "/b"}, rec.commands[0].Argv[1+len(hostFlags):])
	assert.Equal(t, []string{"bundle", "unbundle", "/b"}, rec.commands[1].Argv[1+len(hostFlags):])
}

// TestParseHeads takes "<oid> <ref>" lines and refuses any other shape,
// a ref outside refs/, an object id that is not full and a ref named
// twice.
func TestParseHeads(t *testing.T) {
	a, b := strings.Repeat("a", 40), strings.Repeat("b", 64)
	heads, err := parseHeads(a + " refs/heads/main\n" + b + " refs/romeu/salvage/x\n")
	require.NoError(t, err)
	assert.Equal(t, []Head{{"refs/heads/main", a}, {"refs/romeu/salvage/x", b}}, heads)
	heads, err = parseHeads("")
	require.NoError(t, err)
	assert.Empty(t, heads)
	for _, bad := range []string{
		a + "\n",
		a + " HEAD\n",
		"abc refs/heads/main\n",
		a + " refs/heads/../x\n",
		a + " refs/heads/main\n" + a + " refs/heads/main\n",
	} {
		_, err := parseHeads(bad)
		require.ErrorIs(t, err, ErrRefused, "%q", bad)
	}
}
