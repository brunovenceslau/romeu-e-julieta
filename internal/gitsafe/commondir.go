// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import "context"

// CommonDir returns the absolute common git dir of the repository in
// dir, which a linked worktree shares with its main one, under the
// flags of the mode, with git rev-parse: a snapshot reads the reflog of
// a worktree only when its common git dir is its repository's own. A
// path that is not absolute, valid UTF-8, or that holds a character
// unsafeRune refuses is refused.
func (r *Runner) CommonDir(ctx context.Context, dir string) (string, error) {
	if err := checkRepo(dir); err != nil {
		return "", err
	}
	out, err := r.readLine(ctx, dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if err := checkWorktreePath(out); err != nil {
		return "", err
	}
	return out, nil
}
