// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// The unit tests of step 4 run real git on local repositories, with
// fixtures in place of the clone (cloneRepo) and the fetch (fetch):
// gitsafe clones and fetches https URLs only, and the real ones are the
// e2e's.

// The URLs of the two repos of the project shop of treeRun.
const (
	apiURL = "https://github.com/owner/shop-api"
	webURL = "https://github.com/owner/shop-web"
)

// hostTree is a project shop of two repos for step 4: shop-api, the
// primary, whose spec names the branch main, and shop-web, whose spec
// names no ref and whose origin's HEAD is the branch trunk.
type hostTree struct {
	root, env string
	// origins are the local repositories behind each URL.
	origins map[string]string
	r       *run
}

// treeRun returns the fixture of a run whose step 3 is done, the
// target shop, with nothing of its tree made yet, and replaces
// cloneRepo and fetch with a clone and a fetch from the local origins.
func treeRun(t *testing.T) *hostTree {
	t.Helper()
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	git, err := procgroup.LookPath("git")
	require.NoError(t, err)
	h := &hostTree{root: filepath.Join(tmp, "root"), origins: map[string]string{}}
	h.env = filepath.Join(h.root, "shop-env")
	home := filepath.Join(tmp, "home")
	for _, d := range []string{h.root, home} {
		require.NoError(t, os.MkdirAll(d, 0o700))
	}
	for url, branch := range map[string]string{apiURL: "main", webURL: "trunk"} {
		o := filepath.Join(tmp, "origin-"+filepath.Base(url))
		require.NoError(t, os.MkdirAll(o, 0o700))
		gitIn(t, o, "init", "-q", "-b", branch)
		commitFiles(t, o, map[string]string{"README": url})
		h.origins[url] = o
	}
	settings := spec.HostSettings{
		Schema: "host-settings/v1",
		Root:   h.root,
		Config: spec.ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"},
		Tools:  spec.Tools{Sbx: "/usr/bin/false", Git: git},
	}
	h.r = newRun(&cli.Env{}, flags{from: fromOrigin})
	h.r.settings, h.r.home = settings, home
	h.r.git, err = hostGit(settings, home)
	require.NoError(t, err)
	h.r.targets = []spec.Project{{Name: "shop", Repos: []spec.Repo{
		{URL: apiURL, Primary: true, Dir: "shop-api", Ref: "main"},
		{URL: webURL, Dir: "shop-web"},
	}}}
	oldClone, oldFetch := cloneRepo, fetch
	cloneRepo = func(_ context.Context, _ *gitsafe.Runner, url, dest string) error {
		gitIn(t, filepath.Dir(dest), "clone", "-q", h.origins[url], dest)
		return nil
	}
	fetch = func(_ context.Context, _ *gitsafe.Runner, dir, url string, refspecs []string) error {
		gitIn(t, dir, append([]string{"fetch", "-q", h.origins[url]}, refspecs...)...)
		return nil
	}
	t.Cleanup(func() { cloneRepo, fetch = oldClone, oldFetch })
	return h
}

// in returns the path p, slash-separated, below the env dir.
func (h *hostTree) in(p string) string { return filepath.Join(h.env, filepath.FromSlash(p)) }

// snapshot returns every node below dir by its path: its mode, and
// the content of a file or the target of a link. Links are not
// followed.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		v := info.Mode().String()
		switch {
		case info.Mode().IsRegular():
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			v += " " + string(data)
		case info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			v += " -> " + target
		}
		out[p] = v
		return nil
	})
	require.NoError(t, err)
	return out
}

// cloneRefs returns the refs of the clone in dir that step 4 must
// never move (I5): refs/heads, refs/tags, refs/remotes and HEAD.
func cloneRefs(t *testing.T, dir string) string {
	t.Helper()
	return gitIn(t, dir, "for-each-ref", "--format=%(refname) %(objectname) %(symref)", "refs/heads", "refs/tags", "refs/remotes") +
		"\nHEAD " + gitIn(t, dir, "symbolic-ref", "HEAD") + " " + gitIn(t, dir, "rev-parse", "HEAD")
}

// mode returns the permission bits of the node at p, not followed.
func mode(t *testing.T, p string) fs.FileMode {
	t.Helper()
	info, err := os.Lstat(p)
	require.NoError(t, err)
	return info.Mode()
}

// TestMakeTreeOnAFirstSync is step 4 of 04 4.2 on a host with nothing
// of the project yet (J2 step 3): the env dir, its marker and the
// memory dirs, each made 0700 with their fixed subdirectories, a clone
// of each repo, each origin fetched into refs/romeu/origin/<dir>/*, and
// the egress commit of each repo, from the spec's ref or, with none,
// the origin's HEAD.
func TestMakeTreeOnAFirstSync(t *testing.T) {
	h := treeRun(t)

	require.NoError(t, makeTree(t.Context(), h.r))

	assert.Equal(t, fs.ModeDir|0o700, mode(t, h.env))
	assert.True(t, mode(t, h.in(neverIndex)).IsRegular(), "the marker is a regular file")
	for _, d := range []string{"memory", "memory/shop-api", "memory/shop-api/entries", "memory/shop-api/snapshot", "memory/shop-api/salvage",
		"memory/shop-web", "memory/shop-web/entries", "memory/shop-web/snapshot", "memory/shop-web/salvage",
		"memory/_shared", "memory/_shared/entries"} {
		assert.Equal(t, fs.ModeDir|0o700, mode(t, h.in(d)), d)
	}
	assert.NoDirExists(t, h.in("memory/shop-api/handoff"), "a repo dir gets no handoff/: it is the layout of before")
	apiHead := gitIn(t, h.origins[apiURL], "rev-parse", "HEAD")
	webHead := gitIn(t, h.origins[webURL], "rev-parse", "HEAD")
	assert.Equal(t, apiHead, gitIn(t, h.in("shop-api"), "rev-parse", "refs/romeu/origin/shop-api/main"))
	assert.Equal(t, webHead, gitIn(t, h.in("shop-web"), "rev-parse", "refs/romeu/origin/shop-web/trunk"))
	assert.Equal(t, []tree{{
		env:    h.env,
		shared: h.in("memory/_shared"),
		repos: []repoTree{
			{dir: "shop-api", clone: h.in("shop-api"), memory: h.in("memory/shop-api"), egressRef: "refs/romeu/origin/shop-api/main", egressCommit: apiHead},
			{dir: "shop-web", clone: h.in("shop-web"), memory: h.in("memory/shop-web"), egressRef: "refs/romeu/origin/shop-web/trunk", egressCommit: webHead},
		},
	}}, h.r.trees)
	assert.Empty(t, h.r.warnings)
}

// TestMakeTreeAgainChangesNothing: a second step 4 on unchanged inputs
// leaves every node of the env dir as it was (the idempotency rule of
// 04 4.1), and finds the same egress.
func TestMakeTreeAgainChangesNothing(t *testing.T) {
	h := treeRun(t)
	require.NoError(t, makeTree(t.Context(), h.r))
	first, before := h.r.trees, snapshot(t, h.env)
	h.r.trees = nil

	require.NoError(t, makeTree(t.Context(), h.r))

	assert.Equal(t, before, snapshot(t, h.env))
	assert.Equal(t, first, h.r.trees)
}

// TestMakeTreeKeepsWhatExists: on an env dir whose memory dir already
// holds entries, a legacy handoff/ and a snapshot bundle, step 4 keeps
// every node byte for byte with its mode, the modes of the dirs it
// would make 0700 included (create only), and makes only what is
// missing.
func TestMakeTreeKeepsWhatExists(t *testing.T) {
	h := treeRun(t)
	files := map[string]fs.FileMode{
		"memory/shop-api/entries/01j9z3k8m6t2q4w5e7r9y1v3k5.md":       0o644,
		"memory/shop-api/handoff/01j9z3k8m6t2q4w5e7r9y1v3k6-clear.md": 0o600,
		"memory/shop-api/snapshot/repo.bundle":                        0o600,
		neverIndex:                                                    0o644,
	}
	for p, m := range files {
		require.NoError(t, os.MkdirAll(filepath.Dir(h.in(p)), 0o755))
		require.NoError(t, os.WriteFile(h.in(p), []byte("kept "+p), m))
		require.NoError(t, os.Chmod(h.in(p), m))
	}
	require.NoError(t, os.Chmod(h.env, 0o755))
	before := snapshot(t, h.env)

	require.NoError(t, makeTree(t.Context(), h.r))

	after := snapshot(t, h.env)
	for p, v := range before {
		assert.Equal(t, v, after[p], p)
	}
	assert.Equal(t, fs.ModeDir|0o700, mode(t, h.in("memory/shop-api/salvage")), "a missing fixed subdirectory is made")
}

// TestMakeTreeWarnsALegacyHandoff: a repo dir with a handoff/ of before
// 03 3.10's stamp warns handoff-legacy and is not refused (08 8.1,
// REQ-0645).
func TestMakeTreeWarnsALegacyHandoff(t *testing.T) {
	h := treeRun(t)
	require.NoError(t, os.MkdirAll(h.in("memory/shop-web/handoff"), 0o700))

	require.NoError(t, makeTree(t.Context(), h.r))

	var out bytes.Buffer
	for _, w := range h.r.warnings {
		w(cli.NewWriter(&out))
	}
	assert.Equal(t, "warning handoff-legacy: "+h.in("memory/shop-web")+" holds a handoff/ of before the stamp of 03 3.10, which julieta setup migrates\n", out.String())
}

// TestMakeTreeMovesNoRefOfAClone is I5 and I9 on clones that exist: the
// origin moved on, a branch of the clone holds a commit of its own and
// refs/remotes/origin/main was rewritten; step 4 leaves refs/heads,
// refs/tags, refs/remotes and HEAD as they were, and the egress commit
// is the origin's new head, from refs/romeu/origin/<dir>/main alone.
func TestMakeTreeMovesNoRefOfAClone(t *testing.T) {
	h := treeRun(t)
	require.NoError(t, makeTree(t.Context(), h.r))
	clone := h.in("shop-api")
	own := commitFiles(t, clone, map[string]string{"local": "never pushed"})
	gitIn(t, clone, "update-ref", "refs/remotes/origin/main", own)
	gitIn(t, clone, "tag", "kept")
	head := commitFiles(t, h.origins[apiURL], map[string]string{"README": "moved on"})
	before := cloneRefs(t, clone)
	h.r.trees = nil

	require.NoError(t, makeTree(t.Context(), h.r))

	assert.Equal(t, before, cloneRefs(t, clone))
	assert.Equal(t, head, h.r.trees[0].repos[0].egressCommit)
}

// TestMakeTreeRefuses: what step 4 refuses, exit 2 naming the path or
// the ref, with nothing made below a node that is not a plain
// directory and nothing followed out of the tree.
func TestMakeTreeRefuses(t *testing.T) {
	tests := []struct {
		name string
		// arrange plants the case in the fixture; outside is a
		// directory out of the tree, which must stay empty.
		arrange  func(t *testing.T, h *hostTree, outside string)
		slug     string
		mentions func(h *hostTree) string
	}{
		{
			name:     "a branch the origin lacks",
			arrange:  func(_ *testing.T, h *hostTree, _ string) { h.r.targets[0].Repos[0].Ref = "release" },
			slug:     "ref-missing",
			mentions: func(*hostTree) string { return "release" },
		},
		{
			name: "an origin whose HEAD names no branch",
			arrange: func(t *testing.T, h *hostTree, _ string) {
				t.Helper()
				gitIn(t, h.origins[webURL], "symbolic-ref", "HEAD", "refs/heads/gone")
			},
			slug:     "ref-missing",
			mentions: func(*hostTree) string { return webURL },
		},
		{
			name: "an env dir that is a link",
			arrange: func(t *testing.T, h *hostTree, outside string) {
				t.Helper()
				require.NoError(t, os.Symlink(outside, h.env))
			},
			slug:     "tree-unsafe",
			mentions: func(h *hostTree) string { return h.env },
		},
		{
			name: "a memory/ that is a link",
			arrange: func(t *testing.T, h *hostTree, outside string) {
				t.Helper()
				require.NoError(t, os.MkdirAll(h.env, 0o700))
				require.NoError(t, os.Symlink(outside, h.in("memory")))
			},
			slug:     "tree-unsafe",
			mentions: func(h *hostTree) string { return h.in("memory") },
		},
		{
			name: "a memory dir that is a link",
			arrange: func(t *testing.T, h *hostTree, outside string) {
				t.Helper()
				require.NoError(t, os.MkdirAll(h.in("memory"), 0o700))
				require.NoError(t, os.Symlink(outside, h.in("memory/shop-api")))
			},
			slug:     "memory-layout",
			mentions: func(h *hostTree) string { return h.in("memory/shop-api") },
		},
		{
			name: "a shared dir that is a file",
			arrange: func(t *testing.T, h *hostTree, _ string) {
				t.Helper()
				require.NoError(t, os.MkdirAll(h.in("memory"), 0o700))
				require.NoError(t, os.WriteFile(h.in("memory/_shared"), nil, 0o600))
			},
			slug:     "memory-layout",
			mentions: func(h *hostTree) string { return h.in("memory/_shared") },
		},
		{
			name: "a fixed subdirectory that is a link",
			arrange: func(t *testing.T, h *hostTree, outside string) {
				t.Helper()
				require.NoError(t, os.MkdirAll(h.in("memory/shop-web"), 0o700))
				require.NoError(t, os.Symlink(outside, h.in("memory/shop-web/entries")))
			},
			slug:     "memory-layout",
			mentions: func(h *hostTree) string { return h.in("memory/shop-web/entries") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := treeRun(t)
			outside := t.TempDir()
			tt.arrange(t, h, outside)

			err := makeTree(t.Context(), h.r)

			require.Error(t, err)
			assert.Equal(t, 2, cli.ExitCode(err), err.Error())
			assert.Contains(t, err.Error(), tt.slug+": ")
			assert.Contains(t, err.Error(), tt.mentions(h))
			entries, err := os.ReadDir(outside)
			require.NoError(t, err)
			assert.Empty(t, entries, "nothing is made out of the tree")
		})
	}
}
