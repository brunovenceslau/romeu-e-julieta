// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"fmt"
	"slices"
)

// maxReflogEntries is the bound of the entries of one reflog, far
// above what a sandbox holds. It is a variable so the tests can lower
// it.
var maxReflogEntries = 100_000

// Reflog returns the object ids of the reflog entries of ref in the
// repository in dir, oldest first, one per entry, so an id the ref
// held twice is listed twice, under the flags of the mode, with git
// reflog exists and git log -g. ref is HEAD, the HEAD of the worktree
// dir names, or a full ref name that checkDisplayRef accepts. A ref
// with no reflog, a missing ref among them, is an empty list; an entry
// whose commit is gone is left out, as git log -g leaves it. More than
// maxReflogEntries entries are refused.
func (r *Runner) Reflog(ctx context.Context, dir, ref string) ([]string, error) {
	if err := checkRepo(dir); err != nil {
		return nil, err
	}
	if ref != "HEAD" {
		if err := checkDisplayRef(ref); err != nil {
			return nil, err
		}
	}
	// git log -g fails on a ref that does not exist; reflog exists
	// answers both "no ref" and "no reflog" with exit 1.
	// No --end-of-options: git 2.53 and 2.47 accept it here, the floor
	// was not measured, and ref is HEAD or starts with refs/.
	err := r.read(ctx, dir, nil, nil, "reflog", "exists", ref)
	switch {
	case exitedWith(err, 1):
		return nil, nil
	case err != nil:
		return nil, err
	}
	var oids []string
	w := &records{limit: maxStatusRecord, each: func(oid string) error {
		if len(oids) == maxReflogEntries {
			return fmt.Errorf("%w: the reflog of %s holds more than %d entries", ErrRefused, ref, maxReflogEntries)
		}
		if err := checkObjectID(oid); err != nil {
			return err
		}
		oids = append(oids, oid)
		return nil
	}}
	if err := r.read(ctx, dir, nil, lines{w}, "log", "-g", "--format=%H", "--end-of-options", ref, "--"); err != nil {
		return nil, err
	}
	if err := w.close(); err != nil {
		return nil, err
	}
	slices.Reverse(oids) // git log -g lists the newest first
	return oids, nil
}
