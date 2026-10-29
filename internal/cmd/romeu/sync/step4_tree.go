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
// the paths of its host clone and of its memory dir, and the egress
// ref and the commit it points at (I9).
type repoTree struct {
	dir, clone, memory      string
	egressRef, egressCommit string
}

// makeTree is step 4 of 04 4.2 (REQ-0339), for each target: the env
// dir, its .metadata_never_index marker and the memory dirs, each with
// its fixed subdirectories, created 0700 when missing and never
// changed when present (create only); a hardened clone of each missing
// repo; the hardened fetch of each origin into
// refs/romeu/origin/<dir>/*; and the egress commit of each repo, the
// tip of refs/romeu/origin/<dir>/<ref>. A node of the tree that is not
// a plain directory is refused (exit 2) and nothing is made below it,
// so no link is followed (I12). It writes no ref of a clone outside
// refs/romeu/origin/ (I5). A repo memory dir that still holds a
// handoff/ warns handoff-legacy. It fills trees and warnings.
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
	env := filepath.Join(r.settings.Root, p.Name+"-env")
	memory := filepath.Join(env, memoryDir)
	for _, d := range []string{env, memory} {
		if err := makeDir(d, "tree-unsafe"); err != nil {
			return tree{}, err
		}
	}
	if err := makeMarker(filepath.Join(env, neverIndex)); err != nil {
		return tree{}, err
	}
	t := tree{env: env, shared: filepath.Join(memory, sharedDir)}
	if err := makeMemory(t.shared, sharedLayout); err != nil {
		return tree{}, err
	}
	for _, repo := range p.Repos {
		rt := repoTree{dir: repo.Dir, clone: filepath.Join(env, repo.Dir), memory: filepath.Join(memory, repo.Dir)}
		if err := makeMemory(rt.memory, repoLayout); err != nil {
			return tree{}, err
		}
		if legacy(rt.memory) {
			r.warnings = append(r.warnings, func(w *cli.Writer) {
				cli.Warn(w, "handoff-legacy", termsafe.Escape(rt.memory)+" holds a handoff/ of before the stamp of 03 3.10, which julieta setup migrates")
			})
		}
		var err error
		if rt.egressRef, rt.egressCommit, err = fetchRepo(ctx, r.git, repo, rt.clone); err != nil {
			return tree{}, err
		}
		t.repos = append(t.repos, rt)
	}
	return t, nil
}

// makeDir makes the directory at path, mode 0700, when nothing is
// there, and leaves one that is there as it is. Either way it must then
// be a plain directory, never a link: anything else is usage with the
// slug, naming the path.
func makeDir(path, slug string) error {
	if err := os.Mkdir(path, dirMode); err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("the host tree: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("the host tree: %w", err)
	}
	if !info.IsDir() {
		return usage(slug, termsafe.Escape(path)+" is not a directory: romeu makes it, and follows no link in the host tree")
	}
	return nil
}

// makeMarker makes the empty file at path, mode 0600, when nothing is
// there; it never opens what is there, a link included.
func makeMarker(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("the host tree: %w", err)
	}
	return f.Close()
}

// makeMemory makes the memory dir at dir and its subdirectories sub,
// each with makeDir: a node of them that is not a plain directory is
// memory-layout (I24), and step 5 checks the rest of the layout.
func makeMemory(dir string, sub []string) error {
	if err := makeDir(dir, "memory-layout"); err != nil {
		return err
	}
	for _, s := range sub {
		if err := makeDir(filepath.Join(dir, s), "memory-layout"); err != nil {
			return err
		}
	}
	return nil
}

// legacy reports whether the memory dir holds a handoff/ directory of
// before the stamp of 03 3.10 (08 8.1): julieta setup migrates it, and
// until then it is a warning, never a refusal (REQ-0645).
func legacy(dir string) bool {
	info, err := os.Lstat(filepath.Join(dir, memstore.HandoffDir))
	return err == nil && info.IsDir()
}

// fetchRepo clones repo into clone when nothing is there, fetches its
// origin into refs/romeu/origin/<dir>/* and returns the egress ref,
// refs/romeu/origin/<dir>/<ref>, and its commit (I9). With no ref in
// the spec, <ref> is the origin's HEAD as the clone recorded it,
// refs/remotes/origin/HEAD; its commit is still read from
// refs/romeu/origin/ alone.
func fetchRepo(ctx context.Context, g *gitsafe.Runner, repo spec.Repo, clone string) (string, string, error) {
	if _, err := os.Lstat(clone); errors.Is(err, fs.ErrNotExist) {
		if err := cloneRepo(ctx, g, repo.URL, clone); err != nil {
			return "", "", err
		}
	} else if err != nil {
		return "", "", fmt.Errorf("the clone of %s: %w", termsafe.Escape(repo.URL), err)
	}
	prefix := "refs/romeu/origin/" + repo.Dir + "/"
	if err := fetch(ctx, g, clone, repo.URL, []string{"+refs/heads/*:" + prefix + "*"}); err != nil {
		return "", "", err
	}
	ref := repo.Ref
	if ref == "" {
		var err error
		if ref, err = g.DefaultBranch(ctx, clone); err != nil {
			return "", "", err
		}
		if ref == "" {
			return "", "", usage("ref-missing", "the spec names no ref for "+termsafe.Escape(repo.URL)+", and its clone records no HEAD of the origin; name the branch in ref")
		}
	}
	commit, ok, err := g.RefOID(ctx, clone, prefix+ref)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", usage("ref-missing", "the origin of "+termsafe.Escape(repo.URL)+" has no branch "+termsafe.Escape(ref))
	}
	return prefix + ref, commit, nil
}
