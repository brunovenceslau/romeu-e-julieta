// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// tipRepo is the name of the bare repository TipFile makes in the
// directory its caller gives it.
const tipRepo = "tip.git"

// ErrNoDefaultBranch is the error of TipFile for an origin whose HEAD
// names no branch it can read: an empty repository, a HEAD that is not a
// symbolic ref, one that names a ref outside refs/heads/ or a name
// gitsafe does not accept, or an advertisement over outputLimit. It is a
// state of the origin, not a refusal of the caller's input, so it does
// not wrap ErrRefused.
var ErrNoDefaultBranch = errors.New("gitsafe: the origin names no default branch")

// tipFlags are the flags TipFile adds to the flags of sandbox mode
// for its ls-remote and fetch, the hardening host mode has on every
// call (05 5.1): no redirect is followed, so one fails with ErrMoved;
// no automatic gc or maintenance runs in the throwaway repository; and
// every object received is checked. They are TipFile's own, not added
// to sandboxSettings: julieta's other git calls run in the clones the
// agent works in, whose config is the sandbox's own.
var tipFlags = []setting{
	{"http.followRedirects", "false"},
	{"gc.auto", "0"},
	{"maintenance.auto", "false"},
	{"transfer.fsckObjects", "true"},
}

// TipFile returns the bytes of the regular file name in the commit at
// the tip of the branch ref of the repository at url, and whether that
// commit holds one, reading only that commit: a shallow fetch, depth 1,
// of the one branch into a new bare repository, with no checkout and
// no tag. An empty ref is the branch the origin's HEAD names, read with
// git ls-remote --symref. It is what julieta spec validate --catalog
// reads of each repo's mise.lock (04 4.3, REQ-0422). Sandbox mode only.
//
// work is an empty directory the caller made and removes: the bare
// repository is made in it, so nothing is left once the caller removes
// it. The URL passes the rules of a fetch (CheckURL, and the same check
// of what the config rewrites it to), the refspec writes refs/remotes/
// only (I5), and the file is read as CommitFile reads it, under limit:
// a file that is a symbolic link, a gitlink, a directory or over limit
// is refused, ErrRefused and ErrFileRefused. The tip is peeled to its
// commit (<oid>^{commit}) before the file is read. The ls-remote and
// the fetch run with tipFlags.
//
// Deferred (2026-10-30): depth 1 bounds the commits fetched, not the
// bytes of the tip's tree; --filter=blob:limit=<cap>, which works with
// GIT_NO_LAZY_FETCH since the blobs under the cap still come, is the
// change once a tip tree is too large for the config repo's CI.
//
// No --filter=blob:none: with GIT_NO_LAZY_FETCH set on every call, the
// blob of the file would then be missing and never fetched, so the
// fetch takes the one commit's tree and blobs whole.
func (r *Runner) TipFile(ctx context.Context, work, url, ref, name string, limit int) ([]byte, bool, error) {
	if r.mode != Sandbox {
		return nil, false, fmt.Errorf("%w: the fetch of a tip in %s mode", ErrRefused, r.mode)
	}
	if err := CheckURL(url); err != nil {
		return nil, false, err
	}
	if !filepath.IsAbs(work) || filepath.Clean(work) != work || work == "/" {
		return nil, false, fmt.Errorf("%w: the directory %q is not an absolute, clean path below /", ErrRefused, work)
	}
	dest := filepath.Join(work, tipRepo)
	if err := checkCloneDest(dest); err != nil {
		return nil, false, err
	}
	if err := r.runWith(ctx, work, nil, nil, "init", "-q", "--bare", "--template=", "--", dest); err != nil {
		return nil, false, err
	}
	resolved, err := r.resolveURL(ctx, dest, url)
	if err != nil {
		return nil, false, err
	}
	if ref == "" {
		if ref, err = r.remoteDefault(ctx, dest, url, resolved); err != nil {
			return nil, false, err
		}
	}
	if err := checkRefName("refs/heads/"+ref, false); err != nil {
		return nil, false, err
	}
	tracking := "refs/remotes/origin/" + ref
	spec := "+refs/heads/" + ref + ":" + tracking
	if err := checkRefspec(r.mode, spec); err != nil {
		return nil, false, err
	}
	args := slices.Concat([]string{"fetch"}, fetchFlags, []string{"--depth=1", url, spec})
	if err := r.runWith(ctx, dest, r.tipSettings(url), nil, args...); err != nil {
		return nil, false, movedError(err, url, resolved)
	}
	oid, ok, err := r.RefOID(ctx, dest, tracking)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, fmt.Errorf("gitsafe: the fetch of %s wrote no %s", ref, tracking)
	}
	out, err := r.output(ctx, dest, nil, "rev-parse", "--verify", "-q", "--end-of-options", oid+"^{commit}")
	if err != nil {
		return nil, false, fmt.Errorf("gitsafe: the tip of %s is no commit: %w", ref, err)
	}
	commit := strings.TrimSuffix(out, "\n")
	if err := checkObjectID(commit); err != nil {
		return nil, false, err
	}
	return r.CommitFile(ctx, dest, commit, name, limit)
}

// remoteDefault returns the branch the HEAD of the origin at url names,
// read with git ls-remote --symref under tipFlags, its output
// bounded by outputLimit. An origin whose HEAD is no symbolic ref to a
// branch gitsafe accepts, or whose advertisement passes outputLimit, is
// ErrNoDefaultBranch.
func (r *Runner) remoteDefault(ctx context.Context, dir, url, resolved string) (string, error) {
	out := limitedBuffer{limit: outputLimit}
	if err := r.runWith(ctx, dir, r.tipSettings(url), &out, "ls-remote", "--symref", "--", url, remoteHEAD); err != nil {
		return "", movedError(err, url, resolved)
	}
	if out.truncated {
		return "", fmt.Errorf("%w: %q advertises more than %d bytes for HEAD", ErrNoDefaultBranch, url, outputLimit)
	}
	for line := range strings.Lines(out.String()) {
		target, ok := strings.CutPrefix(strings.TrimSuffix(line, "\n"), "ref: ")
		if !ok {
			continue
		}
		target, name, ok := strings.Cut(target, "\t")
		if !ok || name != remoteHEAD {
			continue
		}
		branch, ok := strings.CutPrefix(target, "refs/heads/")
		if !ok || checkRefName(target, false) != nil {
			return "", fmt.Errorf("%w: %q advertises its HEAD as %q", ErrNoDefaultBranch, url, clipName(line))
		}
		return branch, nil
	}
	return "", fmt.Errorf("%w: %q", ErrNoDefaultBranch, url)
}

// tipSettings returns the extra flags of TipFile's network calls for
// url: tipFlags, then the settings of the URL's host.
func (r *Runner) tipSettings(url string) []setting {
	return slices.Concat(tipFlags, r.hostSettings(url))
}
