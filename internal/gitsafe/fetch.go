// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"fmt"
	"slices"
)

// fetchFlags are the flags of every fetch (05 5.1): a fetch that
// brings nothing then writes nothing, which the idempotency rule of 04
// 4.1 needs.
var fetchFlags = []string{"--no-tags", "--no-prune", "--no-recurse-submodules", "--no-write-fetch-head"}

// Fetch fetches the refspecs from url into the repository in dir. The
// URL is explicit, never a remote name, and both it and what the
// repository's config rewrites it to must pass CheckURL; each refspec
// must write into a fetch destination of the mode (I5).
func (r *Runner) Fetch(ctx context.Context, dir, url string, refspecs []string) error {
	if err := checkRepo(dir); err != nil {
		return err
	}
	if len(refspecs) == 0 {
		return fmt.Errorf("%w: a fetch of %q names no refspec", ErrRefused, url)
	}
	for _, spec := range refspecs {
		if err := checkRefspec(r.mode, spec); err != nil {
			return err
		}
	}
	if _, err := r.resolveURL(ctx, dir, url); err != nil {
		return err
	}
	args := slices.Concat([]string{"fetch"}, fetchFlags, []string{url}, refspecs)
	return r.runWith(ctx, dir, r.hostSettings(url), nil, args...)
}

// FastForward moves the branch checked out in the sandbox clone in dir
// forward to oid, and fails when that is not a fast-forward. Whether
// to move it, a clean worktree among the conditions, is the caller's
// rule (04 4.3, julieta setup). The host never moves a branch (I5), so
// host mode refuses it.
//
// Three things the caller's checks do not see are guarded here. An
// ignored, untracked file at a path the move would write a tracked
// file to, which git merge overwrites by default, makes git fail and
// name the path (--no-overwrite-ignore), so the move destroys no work.
// The merge verifies no signature (--no-verify-signatures), so
// merge.verifySignatures in the config never starts gpg.program or
// gpg.ssh.program. And the checkout runs no filter: as Status does,
// FastForward first lists the filter drivers the config defines and
// runs the merge with filterNeutral on each (filterOverrides, whose
// precondition applies).
func (r *Runner) FastForward(ctx context.Context, dir, oid string) error {
	if r.mode != Sandbox {
		return fmt.Errorf("%w: fast-forward in %s mode", ErrRefused, r.mode)
	}
	if err := checkRepo(dir); err != nil {
		return err
	}
	if err := checkObjectID(oid); err != nil {
		return err
	}
	noFilters, err := r.filterOverrides(ctx, dir)
	if err != nil {
		return err
	}
	return r.runInput(ctx, dir, nil, noFilters, nil, nil, "merge", "--ff-only", "--no-stat", "--no-overwrite-ignore", "--no-verify-signatures", oid)
}
