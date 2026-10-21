// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package write holds Store, the writer of a memory dir (08 8.1,
// REQ-0653), which julieta alone links (I24: romeu reads memory only
// through the readers of internal/memstore). Store stamps the id and
// the times of an entry (REQ-0656), takes the store lock for every
// write (REQ-0655) and writes each file by the atomic write rule of 03
// 3.8, through a temporary file .tmp-<ULID> in the target's directory
// (REQ-0654). It never deletes an entry: julieta memory edit --status
// done closes one. The clock and the ULID source are injected (10
// 10.4), and every ULID comes from canon.NewULID.
package write

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"time"

	"golang.org/x/sys/unix"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// locksDir is the directory of the store locks, below $HOME
// (REQ-0655): sandbox-local, never in the memory dir.
const locksDir = ".local/state/julieta/locks"

// LockPath returns the store lock of the memory dir of the repo dir,
// $HOME/.local/state/julieta/locks/<dir>.lock, with home as $HOME.
func LockPath(home, dir string) string {
	return filepath.Join(home, locksDir, dir+".lock")
}

// ErrFull is the error of an add to a memory dir that holds
// memstore.MaxEntries entries already (08 8.6).
var ErrFull = errors.New("memstore: the memory dir holds the most entries it may")

// Config is what Open needs: the memory dir, the store lock, and the
// clock and ULID source the store stamps with.
type Config struct {
	// Dir is the memory dir, an absolute and clean path.
	Dir string
	// Lock is the store lock, LockPath of the repo.
	Lock string
	// Now is the clock.
	Now func() time.Time
	// ULIDs is the source of every ULID the store writes.
	ULIDs *canon.ULIDSource
}

// Store is the writer of one memory dir: Add and Edit, each under the
// store lock.
type Store struct {
	*memstore.Reader
	lock  string
	now   func() time.Time
	ulids *canon.ULIDSource
}

// Open returns the store of c.Dir. The caller closes it.
func Open(c Config) (*Store, error) {
	if c.Now == nil || c.ULIDs == nil {
		return nil, errors.New("memstore: a store needs a clock and a ULID source")
	}
	if !filepath.IsAbs(c.Lock) || filepath.Clean(c.Lock) != c.Lock {
		return nil, fmt.Errorf("memstore: the store lock %q is not an absolute, clean path", c.Lock)
	}
	r, err := memstore.Open(c.Dir)
	if err != nil {
		return nil, err
	}
	return &Store{Reader: r, lock: c.Lock, now: c.Now, ulids: c.ULIDs}, nil
}

// Add adds an entry of the content c, normalized, stamped with a new
// id and the time of now as both its times (REQ-0656), and returns it
// with wrote set. When an open entry has the same memory-content digest
// (03 3.12), Add writes nothing and returns that entry with wrote unset
// (04 4.1, Idempotency), so a rerun after a timeout adds nothing.
func (s *Store) Add(c memstore.Content) (e memstore.Entry, wrote bool, err error) {
	c = c.Normal()
	if err := c.Check(); err != nil {
		return memstore.Entry{}, false, err
	}
	sum, err := c.Digest()
	if err != nil {
		return memstore.Entry{}, false, err
	}
	err = s.locked(func() error {
		// One entry is in memory at a time: a full store holds up to
		// MaxEntries files of MaxFileSize each.
		found := false
		var derr error
		err := s.Walk(memstore.Filter{Status: memstore.StatusOpen}, func(have memstore.Entry) bool {
			d, err := have.Content().Digest()
			if err != nil {
				derr = fmt.Errorf("%s: %w", memstore.EntryPath(have.ID), err)
				return false
			}
			if d == sum {
				e, found = have, true
				return false
			}
			return true
		})
		if err = errors.Join(err, derr); err != nil || found {
			return err
		}
		n, err := s.Count()
		if err != nil {
			return err
		}
		if n >= memstore.MaxEntries {
			return fmt.Errorf("%w: %d", ErrFull, memstore.MaxEntries)
		}
		now := memstore.Stamp(s.now())
		id, err := canon.NewULID(now, s.ulids)
		if err != nil {
			return err
		}
		e = memstore.Entry{Schema: memstore.Field, ID: id, Title: c.Title, Status: c.Status, Tags: c.Tags,
			CreatedAt: now, UpdatedAt: now, Body: c.Body}
		wrote = true
		return s.put(e)
	})
	if err != nil {
		return memstore.Entry{}, false, err
	}
	return e, wrote, nil
}

// Change is what an edit sets; a nil field keeps the entry's value.
type Change struct {
	Title  *string
	Body   *string
	Tags   *[]string
	Status *string
}

// Edit applies ch to the entry id and stamps updatedAt with the time of
// now (REQ-0656). An edit that leaves the content as it was writes
// nothing and returns the entry with wrote unset, so a rerun converges
// (04 4.1, Idempotency).
func (s *Store) Edit(id string, ch Change) (e memstore.Entry, wrote bool, err error) {
	err = s.locked(func() error {
		e, err = s.Get(id)
		if err != nil {
			return err
		}
		c := e.Content()
		if ch.Title != nil {
			c.Title = *ch.Title
		}
		if ch.Body != nil {
			c.Body = *ch.Body
		}
		if ch.Tags != nil {
			c.Tags = *ch.Tags
		}
		if ch.Status != nil {
			c.Status = *ch.Status
		}
		c = c.Normal()
		if err := c.Check(); err != nil {
			return err
		}
		old := e.Content()
		if c.Title == old.Title && c.Body == old.Body && c.Status == old.Status && slices.Equal(c.Tags, old.Tags) {
			return nil
		}
		e.Title, e.Body, e.Tags, e.Status = c.Title, c.Body, c.Tags, c.Status
		// A clock that went back behind createdAt keeps the times in
		// order.
		e.UpdatedAt = memstore.Stamp(s.now())
		if e.UpdatedAt.Before(e.CreatedAt) {
			e.UpdatedAt = e.CreatedAt
		}
		wrote = true
		return s.put(e)
	})
	if err != nil {
		return memstore.Entry{}, false, err
	}
	return e, wrote, nil
}

// locked runs f under the store lock (REQ-0655).
func (s *Store) locked(f func() error) error {
	unlock, err := Lock(s.lock, nil)
	if err != nil {
		return err
	}
	defer unlock()
	return f()
}

// Lock takes the exclusive flock of the lock file at path, which is
// made 0600, with its directory 0700, when it is absent, so the lock of
// a moved store is recreated on first use (REQ-0647). A second process
// waits for the first; onWait, when not nil, is called once just before
// the wait, when the lock is held by another (a test seam, 10 10.4).
// unlock releases it. The lock is a flock of golang.org/x/sys/unix
// rather than github.com/gofrs/flock (decided on 2026-10-20): no new
// module, one call with O_NOFOLLOW and an EINTR loop, and what
// gofrs/flock adds (Windows, fcntl locks for NFS, shared locks) julieta
// does not need.
func Lock(path string, onWait func()) (unlock func(), err error) {
	return lock(context.Background(), path, onWait, true)
}

// lockPoll is how often LockContext tries the lock again while it
// waits.
const lockPoll = 10 * time.Millisecond

// LockContext is Lock with a wait that ends with ctx: it tries the lock
// every lockPoll until it is free, and returns the error of ctx, the
// lock not taken, when ctx ends first (decided while building T060), so
// a snapshot that a held lock keeps waiting meets its deadline.
func LockContext(ctx context.Context, path string, onWait func()) (unlock func(), err error) {
	return lock(ctx, path, onWait, false)
}

// lock is Lock, with a wait that blocks with block set and polls until
// ctx ends otherwise.
func lock(ctx context.Context, path string, onWait func(), block bool) (unlock func(), err error) {
	if err := os.MkdirAll(filepath.Dir(path), memstore.DirMode); err != nil {
		return nil, err
	}
	lf, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|unix.O_NOFOLLOW, memstore.FileMode)
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
			onWait()
		}
		if block {
			err = flock(unix.LOCK_EX)
		} else {
			err = pollLock(ctx, func() error { return flock(unix.LOCK_EX | unix.LOCK_NB) })
		}
	}
	if err != nil {
		_ = lf.Close()
		return nil, fmt.Errorf("memstore: the store lock %s: %w", path, err)
	}
	return func() {
		_ = flock(unix.LOCK_UN)
		_ = lf.Close()
	}, nil
}

// pollLock calls try every lockPoll until it takes the lock or fails
// otherwise than EWOULDBLOCK, or ctx ends.
func pollLock(ctx context.Context, try func() error) error {
	tick := time.NewTicker(lockPoll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			if err := try(); !errors.Is(err, unix.EWOULDBLOCK) {
				return err
			}
		}
	}
}

// put writes the file of e by the atomic write rule of 03 3.8: entries/
// is made 0700 when absent, the file is written 0600 as a temporary
// file .tmp-<ULID> in entries/, synced, renamed into place, and the
// directory synced.
func (s *Store) put(e memstore.Entry) error {
	data, err := memstore.Encode(e)
	if err != nil {
		return err
	}
	if err := s.mkdir(memstore.EntriesDir); err != nil {
		return err
	}
	return s.atomicWrite(memstore.EntryPath(e.ID), data)
}

// mkdir makes the directory dir of the root 0700 when it is absent; a
// node there that is no directory is an error.
func (s *Store) mkdir(dir string) error {
	root := s.Root()
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

// atomicWrite replaces the file p of the root with data, as put says.
func (s *Store) atomicWrite(p string, data []byte) error {
	return AtomicWrite(s.Root(), p, data, s.now(), s.ulids, false)
}

// AtomicWrite writes the file p of root with data by the atomic write
// rule of 03 3.8 (REQ-0239): a temporary file .tmp-<ULID> in the
// target's own directory (REQ-0654), 0600, synced, renamed into place,
// and the directory synced. With create set, the file is linked into
// place, so a p that exists already is an error that wraps fs.ErrExist
// and is left as it is, with no gap between the check and the write. It is written here rather than taken
// from github.com/google/renameio, which the other atomic writes of the
// product use (decided on 2026-10-20): REQ-0654 names the temporary
// file .tmp-<ULID>, a name julieta memory check, romeu sync and julieta
// setup read, and renameio cannot name its temporary files. It also
// writes through the os.Root of the memory dir.
func AtomicWrite(root *os.Root, p string, data []byte, now time.Time, ulids *canon.ULIDSource, create bool) error {
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
	if create {
		// A hard link fails with EEXIST when p is there, in one step, so
		// two creators of one name cannot both win.
		lerr := root.Link(tmp, p)
		rerr := root.Remove(tmp)
		if lerr != nil {
			return errors.Join(fmt.Errorf("memstore: %s was not created: %w", p, lerr), rerr)
		}
		if rerr != nil {
			return rerr
		}
	} else if err := root.Rename(tmp, p); err != nil {
		return errors.Join(err, root.Remove(tmp))
	}
	d, err := root.Open(dir)
	if err != nil {
		return err
	}
	return errors.Join(d.Sync(), d.Close())
}

// RemoveTemps removes from the memory dir of root every temporary file
// of julieta (REQ-0654) whose ULID is older than before, the start of
// julieta setup, and keeps a newer one, which a julieta that runs now
// may still be writing. A ULID of a time after now no julieta wrote
// yet, so that file is stale too and is removed. It returns the slash
// paths it removed and kept. It reads the names Check reports, so it
// judges every node as Check does.
func RemoveTemps(root *os.Root, before, now time.Time) (removed, kept []string, err error) {
	rep, err := memstore.Check(root)
	if err != nil {
		return nil, nil, err
	}
	for _, w := range rep.Warnings {
		if w.Rule != memstore.RuleTemp {
			continue
		}
		at, _ := memstore.TempTime(path.Base(w.Path))
		if !at.Before(before) && !at.After(now) {
			kept = append(kept, w.Path)
			continue
		}
		if err := root.Remove(w.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return removed, kept, err
		}
		removed = append(removed, w.Path)
	}
	return removed, kept, nil
}
