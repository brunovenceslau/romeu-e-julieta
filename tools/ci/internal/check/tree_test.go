// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// TestHeadTreeOfAPartialClone checks that a tree whose blob the object
// store lacks is an error that names the file and says how to fetch the
// blob.
func TestHeadTreeOfAPartialClone(t *testing.T) {
	upstream := citest.NewTree(t)
	upstream.Commit("fixture")
	clone := citest.PartialClone(t, upstream)
	upstream.Write("docs/new.md", "new\n")
	upstream.Commit("docs: add a page")
	_, err := clone.Run(t.Context(), nil, "fetch", "--quiet", "origin")
	require.NoError(t, err)
	_, err = check.HeadTree(t.Context(), clone, "refs/remotes/origin/HEAD")
	// The clone has no blob at all: the first entry of the tree is said.
	first := upstream.Git("rev-parse", "HEAD:.githooks/pre-push")
	require.ErrorContains(t, err, "read .githooks/pre-push: the object store lacks its blob "+first+`; in a partial clone (one made with --filter), fetch it with "git cat-file -p `+first+`"`)
}

// TestFileInAndCommitOf checks the file of a tree read once, a
// submodule that is no file, and the commit a revision names.
func TestFileInAndCommitOf(t *testing.T) {
	files := []check.TreeFile{{Path: "a", Mode: "100644", Content: []byte("x")}, {Path: "sub", Mode: "160000"}}
	got, found, err := check.FileIn(files, "a")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "x", string(got))
	_, found, err = check.FileIn(files, "b")
	require.NoError(t, err)
	assert.False(t, found)
	_, _, err = check.FileIn(files, "sub")
	require.ErrorContains(t, err, "not a file")

	r := citest.NewTree(t)
	head := r.Commit("fixture")
	got2, err := check.CommitOf(t.Context(), r.Repo, "HEAD")
	require.NoError(t, err)
	assert.Equal(t, head, got2)
	_, err = check.CommitOf(t.Context(), r.Repo, "nonesuch")
	require.Error(t, err)
}
