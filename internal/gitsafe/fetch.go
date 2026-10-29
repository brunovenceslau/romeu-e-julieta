// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// fetchFlags are the flags of every fetch (05 5.1): a fetch that
// brings nothing then writes nothing, which the idempotency rule of 04
// 4.1 needs.
var fetchFlags = []string{"--no-tags", "--no-prune", "--no-recurse-submodules", "--no-write-fetch-head"}

// Fetch fetches the refspecs from url into the repository in dir. The
// URL is explicit, never a remote name, and both it and what the
// repository's config rewrites it to must pass CheckURL; each refspec
// must write into a fetch destination of the mode (I5).
// refs/romeu/origin/<dir>/HEAD is written only by a fetch whose source
// is HEAD, checked by ls-remote just before (checkRemoteHEAD), and no
// fetch into refs/romeu/origin/ mirrors a ref named HEAD
// (originNegatives; decided on 2026-10-29, T183). A server that answers
// with a redirect, which host mode does not follow, fails the fetch
// with ErrMoved.
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
	resolved, err := r.resolveURL(ctx, dir, url)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(refspecs, fetchesHEAD) {
		if err := r.checkRemoteHEAD(ctx, dir, url, resolved); err != nil {
			return err
		}
	}
	args := slices.Concat([]string{"fetch"}, fetchFlags, []string{url}, refspecs)
	args = append(args, originNegatives(refspecs)...)
	return movedError(r.runWith(ctx, dir, r.hostSettings(url), nil, args...), url, resolved)
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

// headLookalikes are the negative refspecs of every name but HEAD
// itself that git's lookup of the source HEAD tries, in the order of
// ref_rev_parse_rules (git refs.c), so the source HEAD reads only the
// remote's HEAD. Negative refspecs came with git 2.29, below the
// floor.
var headLookalikes = []string{"^refs/HEAD", "^refs/tags/HEAD", "^refs/heads/HEAD", "^refs/remotes/HEAD", "^refs/remotes/HEAD/HEAD"}

// originNegatives returns the negative refspecs of a fetch of
// refspecs, each one checkRefspec accepts: none when no refspec writes
// into refs/romeu/origin/; otherwise headLookalikes, then, for each
// pattern into the namespace, its source with the * read as HEAD, the
// one ref the pattern would map onto a ref named HEAD; each once.
func originNegatives(refspecs []string) []string {
	var negatives []string
	for _, spec := range refspecs {
		src, dst, _ := splitRefspec(spec)
		if !strings.HasPrefix(dst, originPrefix) {
			continue
		}
		if negatives == nil {
			negatives = slices.Clone(headLookalikes)
		}
		if derived := "^" + strings.Replace(src, "*", remoteHEAD, 1); strings.Contains(src, "*") && !slices.Contains(negatives, derived) {
			negatives = append(negatives, derived)
		}
	}
	return negatives
}

// fetchesHEAD reports whether spec fetches the remote's HEAD.
func fetchesHEAD(spec string) bool {
	src, _, _ := splitRefspec(spec)
	return src == remoteHEAD
}

// checkRemoteHEAD returns ErrRefused unless the remote at url, which
// resolves to resolved, advertises exactly one HEAD that names a full
// object id, read with ls-remote --symref under the settings of the
// fetch, its output bounded by outputLimit, over which the remote is
// refused too. An unborn HEAD, or one
// that names a missing branch, shows only its ref: line, so the fetch
// is refused before it runs and writes nothing; a <dir>/HEAD of an
// earlier fetch stays as it was. The remote's HEAD can still move
// between the two calls; the fetch then takes where it moved to.
func (r *Runner) checkRemoteHEAD(ctx context.Context, dir, url, resolved string) error {
	out := limitedBuffer{limit: outputLimit}
	if err := r.runWith(ctx, dir, r.hostSettings(url), &out, "ls-remote", "--symref", "--", url, remoteHEAD); err != nil {
		return movedError(err, url, resolved)
	}
	if out.truncated {
		return fmt.Errorf("%w: %q advertises more than %d bytes for HEAD", ErrRefused, url, outputLimit)
	}
	heads := 0
	for line := range strings.Lines(out.String()) {
		oid, name, ok := strings.Cut(strings.TrimSuffix(line, "\n"), "\t")
		if !ok || strings.HasPrefix(oid, "ref: ") || (name != remoteHEAD && !strings.HasPrefix(name, remoteHEAD+"\t")) {
			continue
		}
		if name != remoteHEAD || checkObjectID(oid) != nil {
			return fmt.Errorf("%w: %q advertises its HEAD as %q", ErrRefused, url, clipName(line))
		}
		heads++
	}
	if heads != 1 {
		return fmt.Errorf("%w: %q advertises %d HEAD lines that name a commit, not 1; its HEAD may be unborn or name a missing branch", ErrRefused, url, heads)
	}
	return nil
}
