// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package actions

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore/write"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

// hooksPathSet reports whether the repo dir's own config sets
// core.hooksPath, which bypasses the dispatcher. A value gitsafe
// refuses to read is set all the same.
func hooksPathSet(ctx context.Context, git *gitsafe.Runner, dir string) (bool, error) {
	_, set, err := git.HooksPath(ctx, dir)
	if errors.Is(err, gitsafe.ErrRefused) {
		return true, nil
	}
	return set, err
}

// RecordDisabled makes snapshot/heads.json of the repo c names say
// whether its own config sets core.hooksPath (08 8.4, REQ-0692), and
// returns that. It is what julieta setup runs, where no snapshot is
// taken: under the store lock of the memory dir, a heads.json that
// already says it is left alone, one that says otherwise is rewritten
// with its refs and time kept, and none is made only for a repo that
// is disabled, with no refs and the time of c.Now. A heads.json that
// does not decode is an error and is left as it is, and so is a dir
// that is no work tree. Only c.Git, Dir,
// Memory, Lock, Now and ULIDs are read.
func RecordDisabled(ctx context.Context, c SnapshotConfig) (disabled bool, err error) {
	if !filepath.IsAbs(c.Memory) || filepath.Clean(c.Memory) != c.Memory {
		return false, fmt.Errorf("the memory dir %q is not an absolute, clean path", c.Memory)
	}
	ctx, cancel := context.WithTimeout(ctx, SnapshotTimeout)
	defer cancel()
	unlock, err := write.LockContext(ctx, c.Lock, nil)
	if err != nil {
		return false, err
	}
	defer unlock()
	// Read under the lock, as Snapshot does, so a value read here is the
	// one the file is written with. A dir that is no work tree is no
	// repo to describe: gitsafe refuses it, which hooksPathSet reads as
	// a refused value.
	if _, err := os.Lstat(filepath.Join(c.Dir, ".git")); err != nil {
		return false, fmt.Errorf("%q is not a repository: %w", c.Dir, err)
	}
	disabled, err = hooksPathSet(ctx, c.Git, c.Dir)
	if err != nil {
		return false, err
	}
	root, err := os.OpenRoot(c.Memory)
	if err != nil {
		return disabled, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	headsPath := path.Join(memstore.SnapshotDir, salvage.HeadsFile)
	var heads salvage.Heads
	data, err := root.ReadFile(headsPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if !disabled {
			return false, nil
		}
		heads = salvage.Heads{CreatedAt: c.Now()}
	case err != nil:
		return disabled, err
	default:
		if heads, err = salvage.DecodeHeads(data); err != nil {
			return disabled, err
		}
		if heads.Disabled == disabled {
			return disabled, nil
		}
	}
	heads.Disabled = disabled
	if data, err = salvage.EncodeHeads(heads); err != nil {
		return disabled, err
	}
	if err := mkdirIn(root, memstore.SnapshotDir); err != nil {
		return disabled, err
	}
	return disabled, write.AtomicWrite(root, headsPath, data, c.Now(), c.ULIDs, false)
}
