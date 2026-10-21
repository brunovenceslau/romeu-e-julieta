// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package actions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore/write"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

// SnapshotConfig is one snapshot of one repo (08 8.4): where its
// objects are, where the pair goes, the store lock that serializes it,
// what it leaves out, and the clock and ULID source it stamps with (10
// 10.4).
type SnapshotConfig struct {
	// Git is julieta's runner, in sandbox mode.
	Git *gitsafe.Runner
	// Dir is the repo: the root of its work tree, or of one of its
	// worktrees.
	Dir string
	// Memory is the repo's memory dir, an absolute and clean path.
	Memory string
	// Lock is the store lock of the memory dir (write.LockPath).
	Lock string
	// Work is a directory of julieta state that this repo's snapshots
	// alone use, where the bundle is built: never in the memory dir,
	// whose layout allows no directory gitsafe's temporary repository
	// could be (08 8.1).
	Work string
	// Generation is the manifest's generation, whose salvage refs the
	// snapshot bundles.
	Generation string
	// Bases are the manifest's repos[].base of the repo: the objects
	// they reach are left out of the bundle.
	Bases []string
	// Now is the clock.
	Now func() time.Time
	// ULIDs is the source of the ULIDs of the temporary files.
	ULIDs *canon.ULIDSource
}

// SnapshotResult is what a snapshot wrote.
type SnapshotResult struct {
	// Heads is the snapshot/heads.json it wrote.
	Heads salvage.Heads
	// Path is the path of that file.
	Path string
	// Bundle says that it wrote heads.bundle; false when no object is
	// beyond the bases, and the bundle of an earlier snapshot is then
	// removed.
	Bundle bool
	// Skipped are the inputs it left out (salvage.Pairs).
	Skipped []salvage.Skip
}

// ErrSnapshotPartial is the error of a snapshot that left out an input
// it could not read (salvage.Pairs): the result is returned with it,
// and a hook records the failure (decided on 2026-10-21).
var ErrSnapshotPartial = errors.New("the snapshot left out what it could not read")

// SnapshotTimeout bounds one snapshot, the wait for the store lock
// included (decided while building T060): a commit's hook never waits
// on it longer.
const SnapshotTimeout = 60 * time.Second

// The test seams of Snapshot (10 10.4), nil or the real call outside
// tests: snapshotOnWait is called when the store lock is held by
// another snapshot, just before the wait; snapshotAfterRead after the
// refs are read and before the bundle is made; snapshotBetweenWrites
// after the bundle's rename and before heads.json, where an error
// stands for a kill; snapshotSync syncs a temporary file.
var (
	snapshotOnWait        func()
	snapshotAfterRead     func()
	snapshotBetweenWrites func() error
	snapshotSync          = (*os.File).Sync
)

// The names gitsafe and the snapshot leave in SnapshotConfig.Work:
// gitsafe.BundleOf's temporary repository, and the bundle git writes
// through a lock file beside it.
const (
	workRepoPrefix   = ".gitsafe-bundle-"
	workBundlePrefix = "heads-"
)

// Snapshot takes a snapshot of the repo of c (08 8.4): under the store
// lock of its memory dir (REQ-0690), from before the read of the refs to
// after the last write, it reads what the bundle holds (salvage.Pairs,
// REQ-0688) and whether the repo's own config sets core.hooksPath
// (REQ-0692), builds the bundle in c.Work (gitsafe.BundleOf) and writes
// snapshot/heads.bundle, then snapshot/heads.json, each by the atomic
// write rule of 03 3.8 through a temporary file .tmp-<ULID> of the
// snapshot dir (REQ-0689). A failed write removes its own temporary
// file and leaves the earlier pair as it was (REQ-0691). When no object
// is beyond the bases (gitsafe.Beyond, asked before any bundle is
// made), the earlier bundle is removed in place of the first write;
// when the plan also left an input out, nothing is written, so a
// partial plan never takes the earlier bundle away. A plan that lost a
// namespace of refs (salvage.Plan.LostNamespace) does not replace an
// earlier pair either, and is written only where there is none; a plan
// that lost a reflog source is written. Before it builds,
// Snapshot removes what a killed snapshot left in c.Work. The whole
// snapshot, the wait for the lock included, ends after
// SnapshotTimeout.
//
// The bundle is copied into the memory dir rather than renamed, since
// c.Work and the memory dir are on different file systems; it is
// streamed, so its size never sits in memory, which is why it is not
// written with write.AtomicWrite, whose data is a []byte.
func Snapshot(ctx context.Context, c SnapshotConfig) (res SnapshotResult, err error) {
	if !filepath.IsAbs(c.Memory) || filepath.Clean(c.Memory) != c.Memory {
		return SnapshotResult{}, fmt.Errorf("the memory dir %q is not an absolute, clean path", c.Memory)
	}
	ctx, cancel := context.WithTimeout(ctx, SnapshotTimeout)
	defer cancel()
	unlock, err := write.LockContext(ctx, c.Lock, func() {
		if snapshotOnWait != nil {
			snapshotOnWait()
		}
	})
	if err != nil {
		return SnapshotResult{}, err
	}
	defer unlock()
	now := c.Now()
	if err := clearWork(c.Work); err != nil {
		return SnapshotResult{}, err
	}
	root, err := os.OpenRoot(c.Memory)
	if err != nil {
		return SnapshotResult{}, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	if err := mkdirIn(root, memstore.SnapshotDir); err != nil {
		return SnapshotResult{}, err
	}

	_, disabled, err := c.Git.HooksPath(ctx, c.Dir)
	if errors.Is(err, gitsafe.ErrRefused) {
		// A value gitsafe refuses to read is set all the same.
		disabled, err = true, nil
	}
	if err != nil {
		return SnapshotResult{}, err
	}
	plan, err := salvage.Pairs(ctx, c.Git, c.Dir, c.Generation, c.Bases)
	if err != nil {
		return SnapshotResult{}, err
	}
	if snapshotAfterRead != nil {
		snapshotAfterRead()
	}
	if plan.LostNamespace() {
		// A plan that lost a namespace of refs does not replace an
		// earlier pair (decided while building T060).
		if _, err := root.Lstat(path.Join(memstore.SnapshotDir, salvage.HeadsFile)); err == nil {
			return SnapshotResult{Skipped: plan.Skipped}, partial(plan.Skipped)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return SnapshotResult{}, err
		}
	}
	heads := salvage.Heads{CreatedAt: now, Disabled: disabled, Refs: plan.Refs()}
	data, err := salvage.EncodeHeads(heads)
	if err != nil {
		return SnapshotResult{}, err
	}
	heads, err = salvage.DecodeHeads(data)
	if err != nil {
		return SnapshotResult{}, err
	}

	oids := make([]string, 0, len(plan.Pairs))
	for _, p := range plan.Pairs {
		oids = append(oids, p.OID)
	}
	bundle, err := c.Git.Beyond(ctx, c.Dir, oids, c.Bases)
	if err != nil {
		return SnapshotResult{}, err
	}
	if !bundle && len(plan.Skipped) > 0 {
		return SnapshotResult{Skipped: plan.Skipped}, partial(plan.Skipped)
	}
	u, err := canon.NewULID(now, c.ULIDs)
	if err != nil {
		return SnapshotResult{}, err
	}
	built := filepath.Join(c.Work, workBundlePrefix+u.String()+".bundle")
	defer func() {
		if rmErr := os.Remove(built); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			err = errors.Join(err, rmErr)
		}
	}()
	if bundle {
		if err := c.Git.BundleOf(ctx, c.Dir, built, plan.Pairs, c.Bases); err != nil {
			return SnapshotResult{}, err
		}
	}

	bundlePath := path.Join(memstore.SnapshotDir, salvage.BundleFile)
	if bundle {
		err = copyAtomic(ctx, root, bundlePath, built, now, c.ULIDs)
	} else {
		err = removeSynced(root, bundlePath)
	}
	if err != nil {
		return SnapshotResult{}, err
	}
	if snapshotBetweenWrites != nil {
		if err := snapshotBetweenWrites(); err != nil {
			return SnapshotResult{}, err
		}
	}
	headsPath := path.Join(memstore.SnapshotDir, salvage.HeadsFile)
	if err := write.AtomicWrite(root, headsPath, data, now, c.ULIDs, false); err != nil {
		return SnapshotResult{}, err
	}
	res = SnapshotResult{Heads: heads, Path: filepath.Join(c.Memory, filepath.FromSlash(headsPath)), Bundle: bundle, Skipped: plan.Skipped}
	if len(plan.Skipped) > 0 {
		return res, partial(plan.Skipped)
	}
	return res, nil
}

// partial returns ErrSnapshotPartial joined with each input left out.
func partial(skipped []salvage.Skip) error {
	errs := []error{ErrSnapshotPartial}
	for _, s := range skipped {
		errs = append(errs, fmt.Errorf("%s: %w", s.Item, s.Err))
	}
	return errors.Join(errs...)
}

// clearWork makes the snapshot's directory of julieta state, 0700 on
// every run, and removes what a killed snapshot left in it: gitsafe's
// temporary repository and a bundle, with git's lock file. The caller
// holds the store lock, so no other snapshot of the repo uses them.
func clearWork(dir string) error {
	if err := os.MkdirAll(dir, memstore.DirMode); err != nil {
		return err
	}
	if err := os.Chmod(dir, memstore.DirMode); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), workRepoPrefix) || strings.HasPrefix(e.Name(), workBundlePrefix) {
			if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// mkdirIn makes the directory dir of the root 0700 when it is absent; a
// node there that is no directory is an error.
func mkdirIn(root *os.Root, dir string) error {
	err := root.Mkdir(dir, memstore.DirMode)
	if err == nil || !errors.Is(err, fs.ErrExist) {
		return err
	}
	info, err := root.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", memstore.ErrLayout, dir)
	}
	return nil
}

// copyAtomic replaces the file p of the root with a copy of the file
// src by the atomic write rule of 03 3.8, as write.AtomicWrite does
// with a []byte: a temporary file .tmp-<ULID> in the target's own
// directory (REQ-0654), 0600, synced, renamed into place, and the
// directory synced. A failure removes the temporary file. src must be
// a regular file, judged with Lstat. The copy stops when ctx ends.
func copyAtomic(ctx context.Context, root *os.Root, p, src string, now time.Time, ulids *canon.ULIDSource) error {
	if info, err := os.Lstat(src); err != nil {
		return err
	} else if !info.Mode().IsRegular() {
		return fmt.Errorf("the bundle %s is not a regular file", src)
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	u, err := canon.NewULID(now, ulids)
	if err != nil {
		return err
	}
	dir := path.Dir(p)
	tmp := path.Join(dir, memstore.TempName(u))
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL|unix.O_NOFOLLOW, memstore.FileMode)
	if err != nil {
		return err
	}
	_, werr := io.Copy(f, ctxReader{ctx, in})
	if err := errors.Join(werr, f.Chmod(memstore.FileMode), snapshotSync(f), f.Close()); err != nil {
		return errors.Join(err, root.Remove(tmp))
	}
	if err := root.Rename(tmp, p); err != nil {
		return errors.Join(err, root.Remove(tmp))
	}
	return syncDir(root, dir)
}

// ctxReader reads r until ctx ends, then fails with the error of ctx.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

// Read reads from r, or returns the error of ctx once it ended.
func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

// removeSynced removes the file p of the root, if it is there, and
// syncs its directory.
func removeSynced(root *os.Root, p string) error {
	if err := root.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return syncDir(root, path.Dir(p))
}

// syncDir syncs the directory dir of the root.
func syncDir(root *os.Root, dir string) error {
	d, err := root.Open(dir)
	if err != nil {
		return err
	}
	return errors.Join(d.Sync(), d.Close())
}
