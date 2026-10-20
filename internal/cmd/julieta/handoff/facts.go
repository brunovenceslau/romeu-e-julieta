// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff

import (
	"context"
	"fmt"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	hf "github.com/brunovenceslau/romeu-e-julieta/internal/handoff"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
)

// gather reads the git facts of the repo r with git in sandbox mode
// (05 5.1): the branch and head, the upstream and the commits ahead and
// behind it, the dirty count and the stashes (03 3.10).
//
// Dirty is the number of kinds of change the working tree holds, of
// staged, unstaged, untracked, unmerged and hidden: gitsafe.Status
// reports those and no count of files, and the file count is a
// follow-up for gitsafe. A change in the number of files within one
// kind is not seen.
func gather(ctx context.Context, git *gitsafe.Runner, r manifest.Repo) (hf.RepoFacts, error) {
	fail := func(err error) (hf.RepoFacts, error) {
		return hf.RepoFacts{}, fmt.Errorf("the git facts of the repo %s: %w", esc(r.Dir), err)
	}
	f := hf.RepoFacts{Dir: r.Dir}
	st, err := git.Status(ctx, r.Path)
	if err != nil {
		return fail(err)
	}
	f.Branch, f.Head, f.Dirty = strings.TrimPrefix(st.Branch, "refs/heads/"), st.OID, dirty(st)
	var upstream string
	if st.Branch != "" {
		branches, err := git.Branches(ctx, r.Path)
		if err != nil {
			return fail(err)
		}
		for _, b := range branches {
			if b.Current {
				upstream = b.Upstream
			}
		}
	}
	if upstream != "" {
		f.Upstream = strings.TrimPrefix(strings.TrimPrefix(upstream, "refs/remotes/"), "refs/heads/")
		_, ok, err := git.RefOID(ctx, r.Path, upstream)
		if err != nil {
			return fail(err)
		}
		if ok && f.Head != "" {
			if f.Ahead, f.Behind, err = git.AheadBehind(ctx, r.Path, "HEAD", upstream); err != nil {
				return fail(err)
			}
		}
	}
	stashes, err := git.Stashes(ctx, r.Path)
	if err != nil {
		return fail(err)
	}
	f.Stashes = len(stashes)
	return f, nil
}

// dirty counts the kinds of change st reports.
func dirty(st gitsafe.Status) int {
	n := 0
	for _, b := range []bool{st.Staged, st.Unstaged, st.Untracked, st.Unmerged, st.Hidden} {
		if b {
			n++
		}
	}
	return n
}
