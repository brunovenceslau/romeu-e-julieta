// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check_test

import (
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/stretchr/testify/require"
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
