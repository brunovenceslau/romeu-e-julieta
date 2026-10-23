// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package lock holds the command julieta lock (04 4.3, REQ-0405,
// REQ-0406, REQ-0407): mise lock for the repo of the working directory,
// or of --repo, with the platforms of 07 7.3, and with --check the
// freshness check of the lock, which reads and writes nothing but
// reports. The check is internal/mise's Check; the catalog lookup of
// the warning lock-unknown-tool is catalog_check.go.
package lock

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/mise"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

var _ = cli.Register(func() cli.Command {
	// lock rewrites mise.lock, and a second run with no input change
	// leaves it as it was; --check writes nothing at all.
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.Converges)
	c.Binary, c.Name = "julieta", "lock"
	c.New = func(env *cli.Env) *cobra.Command {
		var repo string
		var check bool
		cmd := &cobra.Command{
			Use:   "lock",
			Short: "lock the tools of a repo's mise.toml, or check that its mise.lock is fresh",
			Example: "julieta lock\n" +
				"julieta lock --repo api\n" +
				"julieta lock --check",
			Long: "Runs mise lock for the repo of the working directory, or of --repo, with --platform linux-x64,linux-arm64 " +
				"and the platforms of the repo's lockfile_platforms setting, and rewrites mise.lock. " +
				"With --check it writes nothing: it fails with exit 6 on a tool of mise.toml with no lock entry, " +
				"on a lockable entry that lacks linux-x64 or linux-arm64, on an entry without a well-formed sha256 checksum " +
				"(unless .julieta/lock-exceptions excuses its tool) and on a file that does not parse. " +
				"It warns lock-unknown-tool, and still passes, for a backend:tool the embedded egress catalog does not have.",
			Args: cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return run(c.Context(), env, repo, check)
			},
		}
		cmd.Flags().StringVar(&repo, "repo", "", "the dir of the repo, as the manifest names it; the repo of the working directory by default")
		cmd.Flags().BoolVar(&check, "check", false, "check that mise.lock is fresh and write nothing")
		return cmd
	}
	return c
}())

// run locks or checks the repo.
func run(ctx context.Context, env *cli.Env, repo string, check bool) error {
	home, err := setup.Home()
	if err != nil {
		return err
	}
	m, _, err := setup.LoadManifest(home)
	if err != nil {
		return err
	}
	r, err := resolve(home, m, repo)
	if err != nil {
		return err
	}
	if check {
		return runCheck(env, r)
	}
	return runLock(ctx, env, m, home, r)
}

// runCheck is lock --check (REQ-0406, REQ-0407): one line for the repo,
// then one line for each finding, on stdout; a finding is
// check-findings (RJ-328), exit 6, whose details list each finding by its
// slug. A lock, a mise.toml or an exceptions file that does not parse is
// a finding of its own (lock-unparsed), and so is an entry without a
// checksum, unless the exceptions file excuses its tool (07 7.3).
func runCheck(env *cli.Env, r manifest.Repo) error {
	rep, raw, err := mise.CheckDir(r.Path)
	if err != nil {
		return unparsed(r, err)
	}
	dir := termsafe.Escape(r.Dir)
	switch {
	case !raw.HasConfig:
		_, err = fmt.Fprintf(env.Stdout, "lock %s no mise.toml\n", dir)
	case len(rep.Findings) == 0:
		_, err = fmt.Fprintf(env.Stdout, "lock %s ok\n", dir)
	default:
		_, err = fmt.Fprintf(env.Stdout, "lock %s %d findings\n", dir, len(rep.Findings))
	}
	if err != nil {
		return err
	}
	var details []cli.Detail
	var slugs []string
	for _, f := range rep.Findings {
		line := strings.TrimPrefix(f.String(), f.Slug+" ")
		if _, err := fmt.Fprintf(env.Stdout, "finding %s %s\n", f.Slug, termsafe.Escape(line)); err != nil {
			return err
		}
		details = append(details, cli.Detail{Key: dir + " " + termsafe.Escape(line), Value: f.Slug})
		slugs = append(slugs, f.Slug)
	}
	if err := warnUnknownTools(env.Stderr, r.Dir, raw); err != nil {
		return err
	}
	if len(details) > 0 {
		return cli.CheckFindings.New(fmt.Sprintf("the mise.lock of %s is not fresh: %d findings: %s", dir, len(details), strings.Join(slugs, ", ")), details...)
	}
	return nil
}

// unparsed returns lock-unparsed (RJ-325) for a file of the repo that
// cannot be read as one: not a regular file, over the cap or not TOML.
func unparsed(r manifest.Repo, err error) error {
	e := cli.LockUnparsed.Wrap(err, "the mise files of "+termsafe.Escape(r.Dir)+" cannot be read")
	e.Fill = map[string]string{"<dir>": r.Dir}
	return e
}

// runLock runs mise lock with the platforms of 07 7.3 (REQ-0405). It
// refuses a mise.lock that is not a regular file before mise writes it.
func runLock(ctx context.Context, env *cli.Env, m manifest.Manifest, home string, r manifest.Repo) error {
	files, err := mise.ReadConfig(r.Path)
	if err != nil {
		return unparsed(r, err)
	}
	dir := termsafe.Escape(r.Dir)
	if !files.HasConfig {
		_, err = fmt.Fprintf(env.Stdout, "lock %s no mise.toml\n", dir)
		return err
	}
	if err := mise.CheckLockTarget(r.Path); err != nil {
		return unparsed(r, err)
	}
	platforms := mise.Platforms(files.Config)
	d, err := mise.New(os.LookupEnv, home, trusted(m))
	if err != nil {
		return err
	}
	if err := d.Lock(ctx, mise.Target{Dir: r.Path, Repo: r.Dir, Sandbox: m.Project, Egress: m.Egress}, platforms); err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "lock %s platforms %s\n", dir, strings.Join(platforms, ","))
	return err
}

// trusted returns the repo paths of the manifest, the only
// configuration paths mise reads (REQ-0426).
func trusted(m manifest.Manifest) []string {
	paths := make([]string, len(m.Repos))
	for i, r := range m.Repos {
		paths[i] = r.Path
	}
	return paths
}

// resolve returns the repo of the manifest the command acts on: the one
// whose dir is repo, or, when repo is empty, the one whose path holds
// the working directory, the deepest when repos nest. Its dir and path
// are checked before they become a path (REQ-0271).
func resolve(home string, m manifest.Manifest, repo string) (manifest.Repo, error) {
	var found *manifest.Repo
	if repo != "" {
		for i := range m.Repos {
			if m.Repos[i].Dir == repo {
				found = &m.Repos[i]
			}
		}
		if found == nil {
			return manifest.Repo{}, fmt.Errorf("no repo of the manifest has the dir %q", repo)
		}
	} else {
		wd, err := os.Getwd()
		if err != nil {
			return manifest.Repo{}, err
		}
		for i := range m.Repos {
			if within(wd, m.Repos[i].Path) && (found == nil || len(m.Repos[i].Path) > len(found.Path)) {
				found = &m.Repos[i]
			}
		}
		if found == nil {
			return manifest.Repo{}, fmt.Errorf("the working directory %s is in no repo of the manifest: run the command in a repo, or name one with --repo", wd)
		}
	}
	if err := setup.CheckRepo(home, m.Project, *found); err != nil {
		return manifest.Repo{}, err
	}
	return *found, nil
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
