// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// The names step 4 makes in an env dir (02 2.3).
const (
	// neverIndex is the marker that keeps Spotlight from parsing the
	// files agents write (REQ-0138), outside every mount.
	neverIndex = ".metadata_never_index"
	// memoryDir holds the memory dirs; sharedDir is the project's
	// shared one in it (08 8.1, REQ-0642).
	memoryDir = "memory"
	sharedDir = "_shared"
	// dirMode is the mode of each directory step 4 makes (REQ-0141,
	// REQ-0646).
	dirMode = 0o700
)

// repoLayout are the fixed subdirectories of a repo's memory dir (08
// 8.1, REQ-0645). sharedLayout is the part of the shared dir's that
// memstore admits today: its stamps/ waits on memstore's layout of the
// shared dir (T174), since Check refuses any other top-level name.
var (
	repoLayout   = []string{memstore.EntriesDir, memstore.SnapshotDir, memstore.SalvageDir}
	sharedLayout = []string{memstore.EntriesDir}
)

// cloneRepo is the hardened clone of a missing repo. A unit test
// replaces it with a clone of a local repository, which gitsafe
// refuses, since it clones https URLs only; the e2e covers the real
// one.
var cloneRepo = func(ctx context.Context, g *gitsafe.Runner, url, dest string) error {
	return g.Clone(ctx, url, dest)
}

// tree is what step 4 made or found of one target's part of the host
// tree (02 2.3): its env dir, its shared memory dir and its repos.
type tree struct {
	env, shared string
	repos       []repoTree
}

// repoTree is one repo of a target, in the order of its spec: its dir,
// the paths of its host clone and of its memory dir, the branch it
// follows, and the egress ref and the commit it points at (I9).
type repoTree struct {
	dir, clone, memory      string
	branch                  string
	egressRef, egressCommit string
}

// makeTree is step 4 of 04 4.2 (REQ-0339), for each target: the env
// dir, its .metadata_never_index marker and the memory dirs, each with
// its fixed subdirectories, created 0700 when missing and never
// changed when present (create only); a hardened clone of each missing
// repo; the hardened fetch of each origin into
// refs/romeu/origin/<dir>/*; and the egress commit of each repo, the
// tip of refs/romeu/origin/<dir>/<ref>. It writes no ref of a clone
// outside refs/romeu/origin/ (I5). A repo memory dir that still holds
// a handoff/ warns handoff-legacy. It fills trees and warnings; a
// refusal leaves the targets before it as it made them.
//
// What it guarantees against links (I12): the env dir must be a plain
// directory, and every other name is made and judged through an
// os.Root opened on it, so nothing is made or read outside the env
// dir; each level of the tree, the clone dirs included, is judged with
// Lstat before anything is made below it or handed to git, and one
// that is not a plain directory, a link included, is refused (exit 2).
func makeTree(ctx context.Context, r *run) error {
	for _, p := range r.targets {
		t, err := makeTarget(ctx, r, p)
		if err != nil {
			return err
		}
		r.trees = append(r.trees, t)
	}
	return nil
}

// makeTarget is step 4 for the target p.
func makeTarget(ctx context.Context, r *run, p spec.Project) (tree, error) {
	name := p.Name + "-env"
	env := filepath.Join(r.settings.Root, name)
	if err := makeDir(os.Mkdir, os.Lstat, env, env, "tree-unsafe"); err != nil {
		return tree{}, err
	}
	rt, err := os.OpenRoot(env)
	if err != nil {
		return tree{}, fmt.Errorf("the host tree: %w", err)
	}
	defer func() { _ = rt.Close() }()
	in := func(rel string) string { return filepath.Join(env, rel) }
	if err := makeDir(rt.Mkdir, rt.Lstat, memoryDir, in(memoryDir), "tree-unsafe"); err != nil {
		return tree{}, err
	}
	if err := makeMarker(rt); err != nil {
		return tree{}, err
	}
	shared := filepath.Join(memoryDir, sharedDir)
	if err := makeMemory(rt, env, shared, sharedLayout); err != nil {
		return tree{}, err
	}
	t := tree{env: env, shared: in(shared)}
	for _, repo := range p.Repos {
		mem := filepath.Join(memoryDir, repo.Dir)
		rp := repoTree{dir: repo.Dir, clone: in(repo.Dir), memory: in(mem)}
		if err := makeMemory(rt, env, mem, repoLayout); err != nil {
			return tree{}, err
		}
		if legacy(rt, mem) {
			r.warnings = append(r.warnings, func(w *cli.Writer) {
				cli.Warn(w, "handoff-legacy", termsafe.Escape(rp.memory)+" holds a handoff/ of before the stamp of 03 3.10, which julieta setup migrates")
			})
		}
		missing, err := cloneMissing(rt, repo.Dir, rp.clone)
		if err != nil {
			return tree{}, err
		}
		if missing {
			if err := cloneRepo(ctx, r.git, repo.URL, rp.clone); err != nil {
				return tree{}, err
			}
		}
		if rp.branch, rp.egressRef, rp.egressCommit, err = fetchRepo(ctx, r.git, repo, rp.clone); err != nil {
			return tree{}, err
		}
		t.repos = append(t.repos, rp)
	}
	return t, nil
}

// makeDir makes the directory name, mode 0700, with mkdir when nothing
// is there, and leaves one that is there as it is. Either way lstat
// must then find a plain directory, never a link: anything else is
// usage with the slug, naming path, the absolute path of name.
func makeDir(mkdir func(string, fs.FileMode) error, lstat func(string) (fs.FileInfo, error), name, path, slug string) error {
	if err := mkdir(name, dirMode); err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("the host tree: %w", err)
	}
	info, err := lstat(name)
	if err != nil {
		return fmt.Errorf("the host tree: %w", err)
	}
	if !info.IsDir() {
		return usage(slug, termsafe.Escape(path)+" is not a directory: romeu makes it, and follows no link in the host tree")
	}
	return nil
}

// makeMarker makes the empty marker of the env dir of rt, mode 0600,
// when nothing is there; it never opens what is there, a link
// included.
func makeMarker(rt *os.Root) error {
	f, err := rt.OpenFile(neverIndex, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("the host tree: %w", err)
	}
	return f.Close()
}

// makeMemory makes the memory dir dir of the env dir env, open as rt,
// and its subdirectories sub, each with makeDir: a node of them that
// is not a plain directory is memory-layout (I24), and step 5 checks
// the rest of the layout.
func makeMemory(rt *os.Root, env, dir string, sub []string) error {
	if err := makeDir(rt.Mkdir, rt.Lstat, dir, filepath.Join(env, dir), "memory-layout"); err != nil {
		return err
	}
	for _, s := range sub {
		d := filepath.Join(dir, s)
		if err := makeDir(rt.Mkdir, rt.Lstat, d, filepath.Join(env, d), "memory-layout"); err != nil {
			return err
		}
	}
	return nil
}

// legacy reports whether the memory dir dir of rt holds a handoff/
// directory of before the stamp of 03 3.10 (08 8.1): julieta setup
// migrates it, and until then it is a warning, never a refusal
// (REQ-0645).
func legacy(rt *os.Root, dir string) bool {
	info, err := rt.Lstat(filepath.Join(dir, memstore.HandoffDir))
	return err == nil && info.IsDir()
}

// cloneMissing reports whether the clone dir name of rt, at the
// absolute path, is missing, so step 4 clones it. One that is there
// must be a plain directory: a link is tree-unsafe, so no fetch
// writes refs/romeu/origin/ into the repository it points at.
func cloneMissing(rt *os.Root, name, path string) (bool, error) {
	info, err := rt.Lstat(name)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return true, nil
	case err != nil:
		return false, fmt.Errorf("the clone %s: %w", termsafe.Escape(path), err)
	case !info.IsDir():
		return false, usage("tree-unsafe", termsafe.Escape(path)+" is not a directory: romeu clones the repo there, and follows no link in the host tree")
	}
	return false, nil
}

// fetchRepo fetches the origin of repo into refs/romeu/origin/<dir>/*
// of clone and returns the branch the repo follows, the egress ref and
// its commit (I9).
//
// With a ref in the spec, the branch is that ref and the egress ref is
// refs/romeu/origin/<dir>/<ref>. With none, the same fetch takes the
// origin's HEAD into refs/romeu/origin/<dir>/HEAD (T183), which is the
// egress ref, so the egress commit is the origin's and no local ref
// moves it, refs/remotes/origin/HEAD among them. The name of the
// branch, which the manifest gives julieta, is still the one the
// clone's refs/remotes/origin/HEAD points at, since the fetch of HEAD
// stores its commit and not the branch it names; a clone that records
// none is ref-missing, before any fetch. A local rewrite of that ref
// changes the name alone, never the egress (reopened when romeu run
// writes the manifest, T046).
func fetchRepo(ctx context.Context, g *gitsafe.Runner, repo spec.Repo, clone string) (branch, egressRef, commit string, err error) {
	prefix := "refs/romeu/origin/" + repo.Dir + "/"
	refspecs := []string{"+refs/heads/*:" + prefix + "*"}
	branch, egressRef = repo.Ref, prefix+repo.Ref
	if branch == "" {
		if branch, err = g.DefaultBranch(ctx, clone); err != nil {
			return "", "", "", err
		}
		if branch == "" {
			return "", "", "", usage("ref-missing", "the spec names no ref for "+termsafe.Escape(repo.URL)+", and its clone records no HEAD of the origin; name the branch in ref")
		}
		egressRef = prefix + "HEAD"
		refspecs = append(refspecs, "+HEAD:"+egressRef)
	}
	if err := fetch(ctx, g, clone, repo.URL, refspecs); err != nil {
		return "", "", "", err
	}
	commit, ok, err := g.RefOID(ctx, clone, egressRef)
	if err != nil {
		return "", "", "", err
	}
	if !ok {
		return "", "", "", usage("ref-missing", "the origin of "+termsafe.Escape(repo.URL)+" has no branch "+termsafe.Escape(branch))
	}
	return branch, egressRef, commit, nil
}
