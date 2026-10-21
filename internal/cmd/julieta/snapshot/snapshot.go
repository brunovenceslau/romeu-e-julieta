// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package snapshot holds the command julieta snapshot (04 4.3,
// REQ-0416) and the handler of the snapshot actions of the hook
// dispatcher (08 8.4, REQ-0686), which it sets with
// actions.HandleSnapshot: each resolves the repos of the manifest and
// takes their snapshots with actions.Snapshot, which writes the pair
// snapshot/heads.bundle and snapshot/heads.json under the store lock.
package snapshot

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks/actions"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore/write"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// workDir is the directory of julieta state below which each repo's
// snapshots build their bundle, one directory per repo dir.
const workDir = "snapshot"

var _ = cli.Register(func() cli.Command {
	// Each run rewrites snapshot/heads.json, whose createdAt moves (04
	// 4.1, Idempotency, REQ-0279).
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.Appends("a rewritten snapshot/heads.json, whose createdAt moves"),
		cli.SnapshotWrites, cli.AppendsEachRun)
	c.Binary, c.Name = "julieta", "snapshot"
	c.New = func(env *cli.Env) *cobra.Command {
		var repo string
		var all bool
		cmd := &cobra.Command{
			Use:   "snapshot",
			Short: "bundle the unpushed work of a repo into its memory dir's snapshot/",
			Example: "julieta snapshot\n" +
				"julieta snapshot --repo api\n" +
				"julieta snapshot --all",
			Long: "Writes snapshot/heads.bundle, then snapshot/heads.json, in the memory dir of the repo of the working directory, " +
				"of --repo, or of every repo of the manifest with --all. The bundle holds the local branches, tags and notes, " +
				"the current generation's refs/salvage/ refs, each detached worktree HEAD and each reflog commit no other ref reaches, " +
				"minus the objects the manifest's base reaches; heads.json lists every ref read. " +
				"The snapshot of a repo waits for the store lock of its memory dir. " +
				"The hook dispatcher takes the same snapshot after post-commit, post-rewrite and post-merge.",
			Args: cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				if repo != "" && all {
					return usage(c, "--repo and --all name the repos two ways: give one")
				}
				return run(c.Context(), env, repo, all)
			},
		}
		cmd.Flags().StringVar(&repo, "repo", "", "the repo dir to snapshot, instead of the repo of the working directory")
		cmd.Flags().BoolVar(&all, "all", false, "snapshot every repo of the manifest")
		return cmd
	}
	return c
}())

// The clock and the entropy of the snapshots the hook dispatcher takes,
// which runs no command of its own: the process's, replaced by tests
// (10 10.4).
var (
	hookNow     = time.Now
	hookEntropy = rand.Reader
)

// The snapshot handler of the hook dispatcher.
var _ = actions.HandleSnapshot(hook)

// hook takes the snapshot of the repo of the manifest whose work tree,
// or one of whose worktrees, is the directory the hook runs in, and
// prints nothing on success, so a commit is not louder for it. A
// directory of no manifest repo, such as a scratch repo, takes no
// snapshot and is no failure; a manifest julieta cannot read is.
func hook(ctx context.Context, in actions.Input) error {
	home, err := setup.Home()
	if err != nil {
		return err
	}
	m, _, err := setup.LoadManifest(home)
	if err != nil {
		return err
	}
	git, err := setup.NewGit(home)
	if err != nil {
		return err
	}
	repo, ok := repoOfDir(ctx, git, m, in.Dir)
	if !ok {
		return nil
	}
	if err := checkRepo(home, m, repo); err != nil {
		return err
	}
	_, err = actions.Snapshot(ctx, config(git, home, m, repo, hookNow, canon.NewULIDSource(hookEntropy)))
	return err
}

// repoOfDir returns the repo of m whose path is dir, or one of whose
// worktrees is, the links of each resolved; false for none. A repo
// whose worktrees git cannot list, such as a secondary not cloned yet,
// is not dir's.
func repoOfDir(ctx context.Context, git *gitsafe.Runner, m manifest.Manifest, dir string) (manifest.Repo, bool) {
	same := func(a, b string) bool {
		if a == b {
			return true
		}
		ra, err1 := filepath.EvalSymlinks(a)
		rb, err2 := filepath.EvalSymlinks(b)
		return err1 == nil && err2 == nil && ra == rb
	}
	for _, r := range m.Repos {
		if same(r.Path, dir) {
			return r, true
		}
	}
	for _, r := range m.Repos {
		worktrees, err := git.Worktrees(ctx, r.Path)
		if err != nil {
			continue
		}
		for _, w := range worktrees {
			if same(w.Path, dir) {
				return r, true
			}
		}
	}
	return manifest.Repo{}, false
}

// run takes the snapshots of the repos the flags name and prints, for
// each, the files it wrote, with the repo (04 4.1, Idempotency,
// REQ-0280). A repo whose snapshot fails is said on stderr and the
// others go on; the command then fails with the first error.
func run(ctx context.Context, env *cli.Env, repo string, all bool) error {
	home, err := setup.Home()
	if err != nil {
		return err
	}
	m, _, err := setup.LoadManifest(home)
	if err != nil {
		return err
	}
	repos, err := resolve(m, repo, all)
	if err != nil {
		return err
	}
	git, err := setup.NewGit(home)
	if err != nil {
		return err
	}
	var first error
	for _, r := range repos {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := checkRepo(home, m, r)
		var res actions.SnapshotResult
		if err == nil {
			res, err = actions.Snapshot(ctx, config(git, home, m, r, env.Now, env.ULIDs))
		}
		if res.Path != "" {
			if werr := written(env.Stdout, res, r.Dir); werr != nil {
				return werr
			}
		}
		if err == nil {
			continue
		}
		err = fmt.Errorf("the snapshot of the repo %s: %w", r.Dir, err)
		if first == nil {
			first = err
			continue
		}
		if _, werr := fmt.Fprintf(env.Stderr, "error: %s\n", esc(err.Error())); werr != nil {
			return werr
		}
	}
	return first
}

// written prints the lines of one snapshot: the bundle, or that there
// is none, and heads.json with its createdAt, each with the repo.
func written(out *cli.Writer, res actions.SnapshotResult, dir string) error {
	bundle := filepath.Join(filepath.Dir(res.Path), salvage.BundleFile)
	line := "wrote " + esc(bundle) + " repo " + esc(dir) + "\n"
	if !res.Bundle {
		line = "no bundle: the base reaches every ref, repo " + esc(dir) + "\n"
	}
	_, err := fmt.Fprintf(out, "%swrote %s createdAt %s repo %s\n", line, esc(res.Path), res.Heads.CreatedAt.Format(time.RFC3339), esc(dir))
	return err
}

// resolve returns the repos of m the flags name: every one with all,
// the one whose dir is repo, or the one whose path holds the working
// directory, the deepest when repos nest.
func resolve(m manifest.Manifest, repo string, all bool) ([]manifest.Repo, error) {
	switch {
	case all:
		return m.Repos, nil
	case repo != "":
		for _, r := range m.Repos {
			if r.Dir == repo {
				return []manifest.Repo{r}, nil
			}
		}
		return nil, fmt.Errorf("no repo of the manifest has the dir %q", esc(repo))
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	var found *manifest.Repo
	for i := range m.Repos {
		if within(wd, m.Repos[i].Path) && (found == nil || len(m.Repos[i].Path) > len(found.Path)) {
			found = &m.Repos[i]
		}
	}
	if found == nil {
		return nil, fmt.Errorf("the working directory %s is in no repo of the manifest: run the command in a repo, or name one with --repo or --all", esc(wd))
	}
	return []manifest.Repo{*found}, nil
}

// within reports whether the directory wd is dir or below it, with the
// symbolic links of either resolved when they can be.
func within(wd, dir string) bool {
	in := func(a, b string) bool {
		rel, err := filepath.Rel(b, a)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	if in(wd, dir) {
		return true
	}
	rwd, err1 := filepath.EvalSymlinks(wd)
	rdir, err2 := filepath.EvalSymlinks(dir)
	return err1 == nil && err2 == nil && in(rwd, rdir)
}

// checkRepo returns why the repo r of m cannot be snapshot: its dir,
// path and URL are checked as setup checks them, and its memory dir
// must be an absolute, clean path (REQ-0271).
func checkRepo(home string, m manifest.Manifest, r manifest.Repo) error {
	if err := setup.CheckRepo(home, m.Project, r); err != nil {
		return err
	}
	if !filepath.IsAbs(r.Memory) || filepath.Clean(r.Memory) != r.Memory {
		return fmt.Errorf("the memory dir %q of the repo %s is not an absolute, clean path", esc(r.Memory), r.Dir)
	}
	return nil
}

// config returns the snapshot of the repo r of m.
func config(git *gitsafe.Runner, home string, m manifest.Manifest, r manifest.Repo, now func() time.Time, ulids *canon.ULIDSource) actions.SnapshotConfig {
	return actions.SnapshotConfig{
		Git: git, Dir: r.Path, Memory: r.Memory, Lock: write.LockPath(home, r.Dir),
		Work: filepath.Join(setup.StateDir(home), workDir, r.Dir), Generation: m.Generation, Bases: r.Base,
		Now: now, ULIDs: ulids,
	}
}

// esc escapes s with termsafe, so a name an agent wrote cannot forge a
// line of the output (I29).
func esc(s string) string { return termsafe.Escape(s) }

// usage returns the error usage (RJ-349) for flags cmd refuses (04 4.1,
// Parsing; exit 2), with the hint of the help of the command.
func usage(cmd *cobra.Command, message string) error {
	e := cli.Usage.New(message)
	binary, command, _ := strings.Cut(cmd.CommandPath(), " ")
	e.Fill = map[string]string{"<binary>": binary, " <command>": ""}
	if command != "" {
		e.Fill[" <command>"] = " " + command
	}
	return e
}
