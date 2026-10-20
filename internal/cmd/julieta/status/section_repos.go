// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package status

import (
	"context"
	"fmt"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
)

// The section of the repos (REQ-0424): per repo, the branch and HEAD,
// every local branch head with its ahead and behind, what the working
// tree holds, the stashes, the worktrees, and whether snapshots are
// disabled by a repo-local core.hooksPath. A read that fails is listed
// in the repo's errors and the others go on, so one repo git cannot
// read, such as one with a ref of an unsafe name, hides nothing of the
// rest.
var _ = register(section{
	name:  "repos",
	order: 20,
	fill: func(ctx context.Context, src *source, rep *Report) {
		for _, m := range src.m.Repos {
			rep.Repos = append(rep.Repos, readRepo(ctx, src, m))
		}
	},
	text: func(out *cli.Writer, rep *Report) error {
		for _, r := range rep.Repos {
			if err := repoText(out, r); err != nil {
				return err
			}
		}
		return nil
	},
})

// readRepo reads the state of the repo m.
func readRepo(ctx context.Context, src *source, m manifest.Repo) Repo {
	r := Repo{Dir: m.Dir, Path: m.Path, Branches: []Branch{}, Dirty: []string{}, Errors: []string{}, Stashes: []Stash{}, Worktrees: []Worktree{}}
	failed := func(what string, err error) {
		r.Errors = append(r.Errors, what+": "+err.Error())
	}
	if err := setup.CheckRepo(src.home, src.m.Project, m); err != nil {
		failed("manifest", err)
		return r
	}
	git, dir := src.git, m.Path
	if st, err := git.Status(ctx, dir); err != nil {
		failed("status", err)
	} else {
		r.Branch, r.Head = strings.TrimPrefix(st.Branch, "refs/heads/"), st.OID
		for _, d := range []struct {
			set  bool
			name string
		}{{st.Staged, "staged"}, {st.Unstaged, "unstaged"}, {st.Untracked, "untracked"}, {st.Unmerged, "unmerged"}, {st.Hidden, "hidden"}} {
			if d.set {
				r.Dirty = append(r.Dirty, d.name)
			}
		}
	}
	if branches, err := git.Branches(ctx, dir); err != nil {
		failed("branches", err)
	} else {
		for _, b := range branches {
			br := Branch{Name: strings.TrimPrefix(b.Name, "refs/heads/"), Head: b.OID, Current: b.Current, Upstream: b.Upstream}
			if b.Upstream != "" {
				if _, ok, err := git.RefOID(ctx, dir, b.Upstream); err != nil {
					failed("upstream of "+br.Name, err)
				} else if !ok {
					br.UpstreamGone = true
				} else if br.Ahead, br.Behind, err = git.AheadBehind(ctx, dir, b.Name, b.Upstream); err != nil {
					failed("ahead and behind of "+br.Name, err)
				}
			}
			r.Branches = append(r.Branches, br)
		}
	}
	if stashes, err := git.Stashes(ctx, dir); err != nil {
		failed("stashes", err)
	} else {
		for _, s := range stashes {
			r.Stashes = append(r.Stashes, Stash{Head: s.OID, Subject: s.Subject})
		}
	}
	if worktrees, err := git.Worktrees(ctx, dir); err != nil {
		failed("worktrees", err)
	} else {
		for _, w := range worktrees {
			r.Worktrees = append(r.Worktrees, Worktree{Path: w.Path, Head: w.OID, Branch: w.Branch,
				Bare: w.Bare, Detached: w.Detached, Locked: w.Locked, Prunable: w.Prunable})
		}
	}
	if hooks, ok, err := git.HooksPath(ctx, dir); err != nil {
		failed("core.hooksPath", err)
	} else {
		r.HooksPath, r.SnapshotsDisabled = hooks, ok
	}
	return r
}

// repoText prints the state of one repo.
func repoText(out *cli.Writer, r Repo) error {
	var b strings.Builder
	fmt.Fprintf(&b, "repo %s  %s\n", r.Dir, r.Path)
	for _, e := range r.Errors {
		fmt.Fprintf(&b, "  error      %s\n", e)
	}
	branch := r.Branch
	if branch == "" {
		branch = "(detached)"
	}
	fmt.Fprintf(&b, "  branch     %s %s\n", branch, r.Head)
	dirty := "clean"
	if len(r.Dirty) > 0 {
		dirty = strings.Join(r.Dirty, " ")
	}
	fmt.Fprintf(&b, "  dirty      %s\n", dirty)
	for _, br := range r.Branches {
		fmt.Fprintf(&b, "  local      %s %s", br.Name, br.Head)
		switch {
		case br.UpstreamGone:
			fmt.Fprintf(&b, " %s gone", br.Upstream)
		case br.Upstream != "":
			fmt.Fprintf(&b, " %s +%d -%d", br.Upstream, br.Ahead, br.Behind)
		}
		b.WriteString("\n")
	}
	for i, s := range r.Stashes {
		fmt.Fprintf(&b, "  stash      stash@{%d} %s %s\n", i, s.Head, s.Subject)
	}
	for _, w := range r.Worktrees {
		fmt.Fprintf(&b, "  worktree   %s %s %s\n", w.Path, w.Head, strings.TrimPrefix(w.Branch, "refs/heads/"))
	}
	snapshots := "enabled"
	if r.SnapshotsDisabled {
		snapshots = "disabled (core.hooksPath " + r.HooksPath + ")"
	}
	fmt.Fprintf(&b, "  snapshots  %s\n", snapshots)
	_, err := out.Write([]byte(b.String()))
	return err
}
