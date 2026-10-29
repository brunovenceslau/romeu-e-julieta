// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
)

// originRefspec fetches origin's branches into its remote-tracking refs,
// the one namespace a sandbox fetch writes (I5).
const originRefspec = "+refs/heads/*:refs/remotes/origin/*"

// syncRepos clones each missing secondary repo and fetches origin in
// every repo, and moves no branch (04 4.3, REQ-0398, REQ-0399): each
// checkout stays at its commit, and the state of its default branch
// against origin's is reported in the repo's entry. A clone that fails
// leaves the sandbox unusable; a fetch that fails is the warning
// fetch-failed.
func syncRepos(ctx context.Context, r *run) {
	for _, m := range r.m.Repos {
		rep := Repo{Dir: m.Dir, Path: m.Path, FastForward: ffNotFetched}
		syncRepo(ctx, r, m, &rep)
		r.report.Repos = append(r.report.Repos, rep)
	}
}

// syncRepo is syncRepos for one repo.
func syncRepo(ctx context.Context, r *run, m manifest.Repo, rep *Repo) {
	if err := CheckRepo(r.home, r.m.Project, m); err != nil {
		rep.Error = err.Error()
		if !m.Primary {
			r.fail("the secondary repo %s cannot be cloned: %v", m.Dir, err)
		}
		return
	}
	if !m.Primary {
		cloned, err := cloneMissing(ctx, r, m)
		if err != nil {
			rep.Error = err.Error()
			r.fail("the secondary repo %s cannot be cloned: %v", m.Dir, err)
			return
		}
		rep.Cloned = cloned
	}
	if err := r.git.Fetch(ctx, m.Path, m.URL, []string{originRefspec}); err != nil {
		rep.Error = err.Error()
		r.warn(FetchFailed, fmt.Sprintf("%s: %v", m.Dir, err))
		return
	}
	rep.Fetched = true
	branchState(ctx, r, m, rep)
}

// cloneMissing clones the secondary repo m when nothing is at its path,
// and reports whether it did. Its parent, $HOME/src/<project>, is made
// first. Something at the path that is not a repository, an empty
// directory or a file, is a clone that could not be made.
func cloneMissing(ctx context.Context, r *run, m manifest.Repo) (bool, error) {
	_, err := os.Lstat(m.Path)
	if err == nil {
		if _, err := os.Lstat(filepath.Join(m.Path, ".git")); err != nil {
			return false, fmt.Errorf("%s exists and is not a repository", m.Path)
		}
		return false, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(m.Path), 0o755); err != nil {
		return false, err
	}
	return true, r.git.Clone(ctx, m.URL, m.Path)
}

// branchState reports the default branch of the repo m against
// origin's tip: current, behind with nothing of its own, ahead,
// diverged, or why it was not compared. It moves nothing.
func branchState(ctx context.Context, r *run, m manifest.Repo, rep *Repo) {
	failed := func(err error) {
		rep.FastForward, rep.Error = ffFailed, err.Error()
	}
	def, err := r.git.DefaultBranch(ctx, m.Path)
	if err != nil {
		failed(err)
		return
	}
	rep.DefaultBranch = def
	if def == "" {
		rep.FastForward = ffNoDefault
		return
	}
	remote := "refs/remotes/origin/" + def
	tip, ok, err := r.git.RefOID(ctx, m.Path, remote)
	switch {
	case err != nil:
		failed(err)
		return
	case !ok:
		rep.FastForward = ffNoDefault
		return
	}
	st, err := r.git.Status(ctx, m.Path)
	switch {
	case err != nil:
		failed(err)
		return
	case st.Branch != "refs/heads/"+def || st.OID == "":
		rep.FastForward = ffNotCheckedOut
		return
	case !st.Clean():
		rep.FastForward = ffNotClean
		return
	case st.OID == tip:
		rep.FastForward = ffCurrent
		return
	}
	ahead, behind, err := r.git.AheadBehind(ctx, m.Path, "HEAD", remote)
	switch {
	case err != nil:
		failed(err)
		return
	case ahead > 0 && behind > 0:
		rep.FastForward = ffDiverged
		return
	case ahead > 0:
		rep.FastForward = ffAhead
		return
	}
	rep.FastForward = ffBehind
}
