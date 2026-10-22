// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// Lock takes romeu.lock, the one flock of a machine (04 4.1, Locking,
// REQ-0283): exclusive and non-blocking, so a second romeu does not
// wait but fails with the error locked (RJ-101). The caller releases
// it with unlock, which a command does when it ends, or just before the
// final exec of run (REQ-0284). The kernel releases the lock when the
// process ends, so a killed romeu leaves none behind.
//
// The lock is a flock of golang.org/x/sys/unix, as the memory store's
// is (internal/memstore/write, decided on 2026-10-20), rather than a
// new module: the standard library has no public file lock, and what
// github.com/gofrs/flock adds (Windows, shared locks) romeu does not
// need.
func (s *Store) Lock() (*Lock, error) {
	lf, err := s.root.OpenFile(lockName, os.O_RDWR|os.O_CREATE, FileMode)
	if err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	for {
		err = unix.Flock(int(lf.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if !errors.Is(err, unix.EINTR) {
			break
		}
	}
	if err != nil {
		_ = lf.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, Locked.New("another romeu is running", cli.Detail{Key: "lock", Value: filepath.Join(s.dir, lockName)})
		}
		return nil, fmt.Errorf("state: the lock %s: %w", lockName, err)
	}
	return &Lock{store: s, file: lf}, nil
}

// Lock is a held romeu.lock. Every writer of the store takes one, so a
// command that writes a record without the lock does not compile
// (REQ-0283); the writers also check that it is held, and of this
// store.
type Lock struct {
	store *Store
	mu    sync.Mutex
	file  *os.File
}

// Unlock releases the lock. It is safe to call more than once.
func (l *Lock) Unlock() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	_ = unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
	_ = l.file.Close()
	l.file = nil
}

// ErrNotLocked is the error of a write given no lock, a lock that was
// released, or the lock of another store: a defect of the caller.
var ErrNotLocked = errors.New("state: a write needs the held lock of its store")

// check returns ErrNotLocked unless l is held and is the lock of s.
func (l *Lock) check(s *Store) error {
	if l == nil || l.store != s {
		return ErrNotLocked
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return ErrNotLocked
	}
	return nil
}
