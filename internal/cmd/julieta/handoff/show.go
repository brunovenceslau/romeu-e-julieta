// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff

import (
	"context"
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	hf "github.com/brunovenceslau/romeu-e-julieta/internal/handoff"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

var _ = cli.Register(func() cli.Command {
	// show --hook records the time of its run and clears a
	// default-branch-moved warning it showed, so a second run converges
	// to the same state but for that time.
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.Converges)
	c.Binary, c.Name, c.Sub = "julieta", "handoff", "show"
	c.New = func(env *cli.Env) *cobra.Command {
		var repo string
		var hook bool
		cmd := &cobra.Command{
			Use:   "show",
			Short: "print the latest narrative handoff, the facts since it and the open memory entries",
			Example: "julieta handoff show\n" +
				"julieta handoff show --hook",
			Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				if !hook {
					out, err := render(cmd.Context(), repo, false, nil)
					if err != nil {
						return err
					}
					_, err = env.Stdout.Write([]byte(out))
					return err
				}
				var moved []setup.Warning
				return runHook(env, HookSessionStart, "julieta handoff show --hook", func() (string, string, error) {
					out, err := render(cmd.Context(), repo, true, &moved)
					return out, repo, err
				}, func(s *setup.State) {
					// The warning this run showed is cleared with the run
					// (REQ-0399, REQ-0665): the first SessionStart after a
					// move shows it, and the second does not.
					s.Warnings = slices.DeleteFunc(s.Warnings, func(w setup.Warning) bool {
						return w.Slug == setup.DefaultBranchMoved && slices.Contains(moved, w)
					})
				})
			},
		}
		cmd.Flags().BoolVar(&hook, "hook", false, "print for SessionStart: the warnings, the fixed memory line and the lessons too, cut at 8 KiB")
		repoFlag(cmd, &repo)
		return cmd
	}
	return c
}())

// render returns the print of show (08 8.2, REQ-0664): of every repo of
// the manifest, or of repo alone. For a hook it also holds julieta's
// warnings, the fixed memory line and the lessons, and is bounded; the
// warnings it shows that setup recorded as default-branch-moved are
// appended to moved, for the caller to clear.
func render(ctx context.Context, repo string, hook bool, moved *[]setup.Warning) (string, error) {
	sb, err := load()
	if err != nil {
		return "", err
	}
	repos := sb.m.Repos
	if repo != "" {
		t, err := sb.target(repo)
		if err != nil {
			return "", err
		}
		repos = []manifest.Repo{t}
	}
	var dirs []hf.Dir
	paths := map[string]string{}
	for _, r := range repos {
		dirs = append(dirs, hf.Dir{Label: r.Dir, Path: r.Memory})
		paths[r.Dir] = r.Path
	}
	latest, err := hf.ReadLatest(dirs)
	if err != nil {
		return "", err
	}
	s := hf.Session{Hook: hook, Latest: latest, Count: counter(ctx, sb.home, paths)}
	for _, d := range dirs {
		if err := addEntries(&s, d); err != nil {
			return "", err
		}
	}
	hf.SortItems(s.Entries)
	hf.SortItems(s.Lessons)
	if hook {
		s.Limit = hf.DefaultLimit
		if s.Warnings, err = warnings(sb.home, moved); err != nil {
			return "", err
		}
	}
	return s.Render(), nil
}

// addEntries adds the open entries of the memory dir d to the session,
// the lessons apart.
func addEntries(s *hf.Session, d hf.Dir) error {
	r, err := memstore.Open(d.Path)
	if err != nil {
		return fmt.Errorf("the memory dir of %s: %w", esc(d.Label), err)
	}
	defer func() { _ = r.Close() }()
	err = r.Walk(memstore.Filter{Status: memstore.StatusOpen}, func(e memstore.Entry) bool {
		it := hf.Item{Repo: d.Label, ID: e.ID, Title: e.Title, Tags: e.Tags, Body: e.Body}
		if slices.Contains(e.Tags, memstore.TagLesson) {
			s.Lessons = append(s.Lessons, it)
		} else {
			s.Entries = append(s.Entries, it)
		}
		return true
	})
	if err != nil {
		return fmt.Errorf("the memory dir of %s: %w", esc(d.Label), err)
	}
	return nil
}

// counter returns the hf.CountFunc over the repos at paths, by dir: the
// commits of the new head that the old one does not have, from git in
// sandbox mode. Without a git it counts nothing.
func counter(ctx context.Context, home string, paths map[string]string) hf.CountFunc {
	git, err := setup.NewGit(home)
	if err != nil {
		return nil
	}
	return func(repo, from, to string) (int, bool) {
		p, ok := paths[repo]
		if !ok {
			return 0, false
		}
		_, behind, err := git.AheadBehind(ctx, p, from, to)
		return behind, err == nil
	}
}

// warnings returns the lines of julieta's warnings, each by its slug
// (REQ-0665): those setup recorded in julieta state, the
// default-branch-moved ones among them (appended to moved), and every
// recorded hook failure, the newest of each hook and repo.
func warnings(home string, moved *[]setup.Warning) ([]string, error) {
	st, err := setup.ReadState(home)
	if err != nil {
		return nil, err
	}
	failures, ferr := hooks.Failures(failuresDir(home))
	var out []string
	for _, w := range st.Warnings {
		if w.Slug == setup.DefaultBranchMoved && moved != nil {
			*moved = append(*moved, w)
		}
		out = append(out, "warning "+esc(w.Slug)+": "+oneLine(w.Message))
	}
	// Every recorded failure, the git hooks' too (08 8.2); a file of the
	// store that cannot be trusted is one more line, not a failed print.
	for _, f := range failures {
		out = append(out, failureLine(f))
	}
	if ferr != nil {
		out = append(out, "hook failure: the store of hook failures is partly unreadable: "+oneLine(ferr.Error()))
	}
	return out, nil
}
