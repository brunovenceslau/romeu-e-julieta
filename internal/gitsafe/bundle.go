// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
)

// BundleCreate writes to file a bundle of the named refs of the
// repository in dir, minus the objects reachable from the bases, so
// the bundle needs the bases as prerequisites (03 3.6, repos[].base).
// The refs are full ref names and the bases full object ids. git writes
// the bundle through a lock file it renames into place; git refuses a
// bundle that would hold no ref.
func (r *Runner) BundleCreate(ctx context.Context, dir, file string, refs, bases []string) error {
	if err := checkRepo(dir); err != nil {
		return err
	}
	if !filepath.IsAbs(file) {
		return fmt.Errorf("%w: bundle file %q is not an absolute path", ErrRefused, file)
	}
	if len(refs) == 0 {
		return fmt.Errorf("%w: a bundle names no ref", ErrRefused)
	}
	for _, ref := range refs {
		if err := checkRefName(ref, false); err != nil {
			return err
		}
	}
	excluded := make([]string, 0, len(bases))
	for _, base := range bases {
		if err := checkObjectID(base); err != nil {
			return err
		}
		excluded = append(excluded, "^"+base)
	}
	return r.run(ctx, dir, nil, slices.Concat([]string{"bundle", "create", file}, refs, excluded)...)
}
