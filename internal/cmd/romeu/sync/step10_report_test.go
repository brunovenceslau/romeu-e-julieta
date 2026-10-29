// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// reportRun returns the fixture of step 4 with the tree of shop made,
// every spec the targets, and stdout read by the test.
func reportRun(t *testing.T) (*hostTree, *strings.Builder) {
	t.Helper()
	h := treeRun(t)
	require.NoError(t, makeTree(t.Context(), h.r))
	h.r.specs = h.r.targets
	var out strings.Builder
	h.r.env = &cli.Env{Stdout: cli.NewWriter(&out), Stderr: cli.NewWriter(&strings.Builder{})}
	return h, &out
}

// TestReportOrphansReportsAndRemovesNothing is step 10 of 04 4.2
// (REQ-0345, REQ-0334): a <name>-env/ no spec names is orphaned, and in
// a project's env dir a clone or a memory dir its spec does not name is
// an orphaned repo dir; names romeu reserves, names with a dot, links
// and files are neither; nothing under the root changes.
func TestReportOrphansReportsAndRemovesNothing(t *testing.T) {
	h, out := reportRun(t)
	for _, d := range []string{
		filepath.Join(h.root, "foo-env", "foo-api"),
		filepath.Join(h.root, "notes"),
		h.in("old-api"),
		h.in("memory/old-api"),
		h.in("review/shop-api"),
		h.in("Ledger"),
		h.in(".cache"),
	} {
		require.NoError(t, os.MkdirAll(d, 0o700))
	}
	require.NoError(t, os.WriteFile(h.in("notes.txt"), []byte("x"), 0o600))
	require.NoError(t, os.Symlink(h.in("old-api"), h.in("linked")))
	require.NoError(t, os.Symlink(filepath.Join(h.root, "foo-env"), filepath.Join(h.root, "bar-env")))
	before := snapshot(t, h.root)

	require.NoError(t, reportOrphans(t.Context(), h.r))

	assert.Equal(t, "orphaned: foo\n"+
		"orphaned repo dir: shop-env/old-api\n"+
		"orphaned repo dir: shop-env/memory/old-api\n", out.String())
	assert.Equal(t, before, snapshot(t, h.root), "nothing is removed or changed")
}

// TestReportOrphansReadsEverySpec: a project whose spec names it is no
// orphan when it is not a target of the run, and a repo dir its spec
// names is no orphaned repo dir there either.
func TestReportOrphansReadsEverySpec(t *testing.T) {
	h, out := reportRun(t)
	require.NoError(t, os.MkdirAll(filepath.Join(h.root, "blog-env", "blog-site"), 0o700))
	h.r.specs = append(h.r.specs, spec.Project{Name: "blog", Repos: []spec.Repo{{URL: "https://github.com/owner/blog-site", Primary: true, Dir: "blog-site"}}})

	require.NoError(t, reportOrphans(t.Context(), h.r))

	assert.Empty(t, out.String())
}

// TestReportOrphansEscapesTheNames: a dir name that holds a control
// byte is printed escaped (I29).
func TestReportOrphansEscapesTheNames(t *testing.T) {
	h, out := reportRun(t)
	require.NoError(t, os.MkdirAll(filepath.Join(h.root, "x\x1b[31m-env"), 0o700))

	require.NoError(t, reportOrphans(t.Context(), h.r))

	assert.NotContains(t, out.String(), "\x1b")
	assert.Contains(t, out.String(), "orphaned: x")
}
