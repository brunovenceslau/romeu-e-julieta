// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"fmt"
)

// pushRefspec is the one refspec of the one push: the memory
// repository's main to the remote's main, never forced (no +).
const pushRefspec = "refs/heads/main:refs/heads/main"

// pushFlags are the flags of the push: no hook (core.hooksPath of host
// mode already names none), no signed push, no submodule and no tag.
var pushFlags = []string{"--quiet", "--no-verify", "--no-signed", "--recurse-submodules=no", "--no-follow-tags"}

// Push pushes refs/heads/main of the memory repository whose git dir
// is gitDir to url, memory.remote of the project spec, as
// refs/heads/main: the one push romeu builds (I18, REQ-0502; 08 8.1,
// REQ-1304). It is host mode only, takes no refspec and never forces,
// so a remote whose main is not an ancestor refuses it, as a remote
// that refuses access does; the caller warns memory-push-failed.
//
// The URL passes CheckURL, and so does what the git dir's config
// rewrites it to (url.<base>.insteadOf); a url.<base>.pushInsteadOf of
// that config, which ls-remote --get-url does not apply, is refused.
// A server that answers with a redirect fails the push with ErrMoved.
//
//romeu:invariant I18 guard
func (r *Runner) Push(ctx context.Context, gitDir, url string) error {
	if r.mode != Host {
		return fmt.Errorf("%w: a push in %s mode", ErrRefused, r.mode)
	}
	if err := checkRepo(gitDir); err != nil {
		return err
	}
	resolved, err := r.resolveURL(ctx, gitDir, url)
	if err != nil {
		return err
	}
	err = r.runInput(ctx, gitDir, nil, nil, nil, nil, "config", "-z", "--name-only", "--get-regexp", `^url\..*\.pushinsteadof$`)
	switch {
	case err == nil:
		return fmt.Errorf("%w: the config of %q rewrites push URLs (url.<base>.pushInsteadOf)", ErrRefused, gitDir)
	case !exitedWith(err, 1):
		return err
	}
	args := append([]string{"push"}, pushFlags...)
	args = append(args, url, pushRefspec)
	return movedError(r.exec(ctx, gitDir, nil, r.hostSettings(url), nil, nil, nil, args...), url, resolved)
}
