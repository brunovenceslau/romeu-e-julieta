// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"fmt"
	"path/filepath"
)

// BundleVerify checks that the bundle at file is a valid bundle whose
// prerequisites the repository in dir holds, under the flags of the
// mode, and writes nothing. file is an absolute path.
func (r *Runner) BundleVerify(ctx context.Context, dir, file string) error {
	if err := checkBundle(dir, file); err != nil {
		return err
	}
	return r.run(ctx, dir, nil, "bundle", "verify", "--quiet", file)
}

// checkBundle returns ErrRefused for a dir that is not the root of a
// repository and for a bundle file that is not an absolute path, so
// neither reaches git as an option or relative to another directory.
func checkBundle(dir, file string) error {
	if err := checkRepo(dir); err != nil {
		return err
	}
	if !filepath.IsAbs(file) {
		return fmt.Errorf("%w: bundle file %q is not an absolute path", ErrRefused, file)
	}
	return nil
}
