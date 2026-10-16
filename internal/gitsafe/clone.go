// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// cloneFlags are the flags of every clone: no tags, as no fetch takes
// any (05 5.1), no submodules (02 2.3), and no template directory, so
// no hook or file of the machine's templates enters the clone.
var cloneFlags = []string{"--no-tags", "--no-recurse-submodules", "--template="}

// Clone clones url into dest, a directory that does not exist yet, in
// a parent directory that does: the caller makes the parent, with the
// mode it needs. Host mode clones the config repo for romeu init and
// each missing repo for romeu sync; sandbox mode clones each missing
// secondary repo for julieta setup (04 4.3).
//
// The URL is explicit and passes the rules of a fetch: CheckURL, and
// the same check of what the config rewrites it to. git clone reads no
// local config of a repository around it, so the rewrite is resolved
// from the root directory, where only the global config applies: none
// in host mode, the sandbox's own in sandbox mode.
//
// The clone is the one call that writes a new repository's HEAD, its
// default branch, the remote-tracking refs of origin and a checkout;
// it writes them only into a new directory. Every later fetch into it
// follows the destinations of its mode (I5).
func (r *Runner) Clone(ctx context.Context, url, dest string) error {
	if err := CheckURL(url); err != nil {
		return err
	}
	if err := checkCloneDest(dest); err != nil {
		return err
	}
	if _, err := r.resolveURL(ctx, "/", url); err != nil {
		return err
	}
	args := slices.Concat([]string{"clone"}, cloneFlags, []string{"--", url, dest})
	return r.runWith(ctx, filepath.Dir(dest), r.hostSettings(url), nil, args...)
}

// checkCloneDest returns ErrRefused, wrapped with the reason, for a
// clone destination that is not an absolute, clean path below the root
// with nothing at it yet, a dangling symlink included, so a clone never
// follows a link out of the parent the caller chose.
func checkCloneDest(dest string) error {
	switch {
	case !filepath.IsAbs(dest) || filepath.Clean(dest) != dest || dest == "/":
		return fmt.Errorf("%w: clone destination %q is not an absolute, clean path below /", ErrRefused, dest)
	case strings.ContainsFunc(dest, isControl):
		return fmt.Errorf("%w: clone destination %q holds a control character", ErrRefused, dest)
	}
	_, err := os.Lstat(dest)
	switch {
	case err == nil:
		return fmt.Errorf("%w: clone destination %q exists", ErrRefused, dest)
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("gitsafe: clone destination %q: %w", dest, err)
	}
	return nil
}
