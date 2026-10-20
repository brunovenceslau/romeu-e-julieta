// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	hf "github.com/brunovenceslau/romeu-e-julieta/internal/handoff"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore/write"
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

// lockFile takes the store lock at lock with write.Lock (REQ-0655),
// calling the onWait seam when it has to wait.
func lockFile(lock string) (unlock func(), err error) {
	return write.Lock(lock, func() {
		if onWait != nil {
			onWait(lock)
		}
	})
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

// atomicWrite creates the file p of the root with data and never
// replaces one (write.AtomicWrite).
func atomicWrite(root *os.Root, p string, data []byte, now time.Time, ulids *canon.ULIDSource) error {
	return write.AtomicWrite(root, p, data, now, ulids, true)
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
