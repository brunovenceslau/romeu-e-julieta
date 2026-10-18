// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import "context"

// readSettings are the extra settings of Fsck and WalkTree: neither
// follows a replace ref (refs/replace/), so both read the objects the
// commit names and the same ones, and fsck fails on a bad file mode.
// No allowlisted fetch destination is under refs/replace/ today; this
// keeps the two checks on one tree if that ever changes.
var readSettings = []setting{
	{"core.useReplaceRefs", "false"},
	{"fsck.badFilemode", "error"},
}

// Fsck runs git fsck --strict in the repository in dir, under the flags
// of the mode: it checks every object of the repository, and that the
// objects commit reaches are all there. It is the first of the two
// independent checks of every tree romeu materializes (05 5.1,
// REQ-0465; I27). WalkTree is the second, and does not rely on it.
// A file mode other than 100644 and 100755, which fsck --strict only
// warns about (badFilemode), fails it too. commit is a full object id.
func (r *Runner) Fsck(ctx context.Context, dir, commit string) error {
	if err := checkRepo(dir); err != nil {
		return err
	}
	if err := checkObjectID(commit); err != nil {
		return err
	}
	return r.runWith(ctx, dir, readSettings, nil, "fsck", "--strict", "--no-dangling", "--no-progress", commit)
}
