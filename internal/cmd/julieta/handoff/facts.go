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
// Dirty is gitsafe.Status's Changed: the number of distinct paths the
// working tree changes, staged, unstaged, unmerged, untracked or hidden
// (03 3.10), which salvage reads as dirty-at-last-facts:<n> (REQ-0237).
func gather(ctx context.Context, git *gitsafe.Runner, r manifest.Repo) (hf.RepoFacts, error) {
	fail := func(err error) (hf.RepoFacts, error) {
		return hf.RepoFacts{}, fmt.Errorf("the git facts of the repo %s: %w", esc(r.Dir), err)
	}
	f := hf.RepoFacts{Dir: r.Dir}
	st, err := git.Status(ctx, r.Path)
	if err != nil {
		return fail(err)
	}
	f.Branch, f.Head, f.Dirty = strings.TrimPrefix(st.Branch, "refs/heads/"), st.OID, st.Changed
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
