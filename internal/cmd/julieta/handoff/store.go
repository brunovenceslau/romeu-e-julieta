// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	hf "github.com/brunovenceslau/romeu-e-julieta/internal/handoff"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// onWait is called when a lock is held by another process and the
// caller is about to wait for it: the test seam of the lock (10 10.4),
// which lets a test know the command has met the held lock without a
// sleep. It is nil outside tests.
var onWait func(lock string)

// beforeGit is called just before writeHandoff's first git read, with
// the lock held: the second test seam, which lets a test tell that the
// lock came first.
var beforeGit func()

// lockFile takes the exclusive flock of the lock file lock, which it
// makes 0600 with its directory 0700 when it is absent, waits as long
// as another process holds it, and returns the function that releases
// it. It is the lock of memstore/write, the same file and the same
// flock, so a handoff write and a memory add or a snapshot of one dir
// run one after the other (REQ-0655); memstore/write does not export
// its lock, so the few lines are here. The flock is that of
// golang.org/x/sys/unix, as write.go decided on 2026-10-20.
func lockFile(lock string) (unlock func(), err error) {
	if err := os.MkdirAll(filepath.Dir(lock), memstore.DirMode); err != nil {
		return nil, err
	}
	lf, err := os.OpenFile(lock, os.O_RDWR|os.O_CREATE|unix.O_NOFOLLOW, memstore.FileMode)
	if err != nil {
		return nil, err
	}
	flock := func(how int) error {
		for {
			err := unix.Flock(int(lf.Fd()), how)
			if !errors.Is(err, unix.EINTR) {
				return err
			}
		}
	}
	err = flock(unix.LOCK_EX | unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		if onWait != nil {
			onWait(lock)
		}
		err = flock(unix.LOCK_EX)
	}
	if err != nil {
		_ = lf.Close()
		return nil, fmt.Errorf("the lock %s: %w", lock, err)
	}
	return func() {
		_ = flock(unix.LOCK_UN)
		_ = lf.Close()
	}, nil
}

// mkdir makes the directory dir of the root 0700 when it is absent; a
// node there that is no directory is an error.
func mkdir(root *os.Root, dir string) error {
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

// atomicWrite creates the file p of the root with data by the atomic
// write rule of 03 3.8 (REQ-0239): a temporary file .tmp-<ULID> in the
// target's own directory (REQ-0654), written 0600, synced, renamed into
// place, and the directory synced. The name must be new: an existing
// file is an error, since a handoff is never rewritten. It is the write
// of memstore/write, which does not export it, and is written here
// rather than taken from github.com/google/renameio for the reason
// given there: renameio cannot name its temporary file .tmp-<ULID>.
func atomicWrite(root *os.Root, p string, data []byte, now time.Time, ulids *canon.ULIDSource) error {
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
	_, werr := f.Write(data)
	if err := errors.Join(werr, f.Chmod(memstore.FileMode), f.Sync(), f.Close()); err != nil {
		return errors.Join(err, root.Remove(tmp))
	}
	if _, err := root.Lstat(p); err == nil {
		return errors.Join(fmt.Errorf("%s exists already", p), root.Remove(tmp))
	}
	if err := root.Rename(tmp, p); err != nil {
		return errors.Join(err, root.Remove(tmp))
	}
	d, err := root.Open(dir)
	if err != nil {
		return err
	}
	return errors.Join(d.Sync(), d.Close())
}

// prune removes the facts files of the memory dir beyond the newest
// hf.KeepFacts (08 8.1, REQ-0649): never the newest facts file and
// never a narrative file, which it does not look at. It returns the
// names it removed.
func prune(root *os.Root) ([]string, error) {
	names, err := hf.Names(root)
	if err != nil {
		return nil, err
	}
	var facts []hf.Name
	for _, n := range names {
		if n.Kind == hf.Facts {
			facts = append(facts, n)
		}
	}
	var removed []string
	for len(facts) > hf.KeepFacts {
		old := facts[0] // oldest first
		facts = facts[1:]
		if err := root.Remove(hf.Path(old)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return removed, err
		}
		removed = append(removed, old.String())
	}
	return removed, nil
}
