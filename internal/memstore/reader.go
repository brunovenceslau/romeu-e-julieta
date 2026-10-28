// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memstore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
)

// Reader reads the entries of one memory dir through an os.Root on it
// (08 8.1, REQ-0653), so no name reaches outside the dir.
type Reader struct {
	dir  string
	root *os.Root
}

// Open returns the reader of the memory dir at dir, an absolute and
// clean path to a directory that is no symlink. The caller closes it.
func Open(dir string) (*Reader, error) {
	if err := checkPath(dir); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnmounted, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s is not a directory", ErrUnmounted, dir)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	return &Reader{dir: dir, root: root}, nil
}

// Close closes the reader's root.
func (r *Reader) Close() error { return r.root.Close() }

// Dir returns the path of the memory dir.
func (r *Reader) Dir() string { return r.dir }

// Root returns the os.Root on the memory dir, which the writer of
// internal/memstore/write shares.
func (r *Reader) Root() *os.Root { return r.root }

// Path returns the path of the file of the entry u.
func (r *Reader) Path(u canon.ULID) string { return r.dir + "/" + EntryPath(u) }

// Get returns the entry whose id is id, a lowercase ULID (03 3.12): a
// name of any other form, such as "..", is refused before it is used.
// The file must be a regular file, judged with Lstat and again on the
// open file, opened without following a symlink and without waiting on
// a FIFO, of at most MaxFileSize bytes, that decodes strictly (Decode)
// and whose id is its name. An absent file is ErrNotFound.
func (r *Reader) Get(id string) (Entry, error) {
	u, err := canon.ParseULID(id)
	if err != nil {
		return Entry{}, fmt.Errorf("%w: the id \"%s\": %w", ErrNotFound, clip(id), err)
	}
	ok, err := r.entriesDir()
	if err != nil {
		return Entry{}, err
	}
	if !ok {
		return Entry{}, fmt.Errorf("%w: %s", ErrNotFound, u)
	}
	return r.get(u)
}

// get is Get for a parsed id, once entriesDir has passed: Get checks
// it for one entry, and ids for every entry Walk reads.
func (r *Reader) get(u canon.ULID) (Entry, error) {
	data, err := r.readFile(EntryPath(u))
	if errors.Is(err, fs.ErrNotExist) {
		return Entry{}, fmt.Errorf("%w: %s", ErrNotFound, u)
	}
	if err != nil {
		return Entry{}, err
	}
	e, err := Decode(data)
	if err != nil {
		return Entry{}, fmt.Errorf("%s: %w", EntryPath(u), err)
	}
	if e.ID != u {
		return Entry{}, fmt.Errorf("%s: %w: the id %s is not the file's name", EntryPath(u), ErrInvalid, e.ID)
	}
	return e, nil
}

// readFile returns the content of the regular file p of the root, read
// as Get says.
func (r *Reader) readFile(p string) ([]byte, error) {
	info, err := r.root.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s is not a regular file", ErrLayout, p)
	}
	f, err := r.root.OpenFile(p, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s is not a regular file", ErrLayout, p)
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFileSize {
		return nil, fmt.Errorf("%w: %s passes the file cap of %d bytes", ErrInvalid, p, MaxFileSize)
	}
	return data, nil
}

// The orders of List (julieta memory list --sort, REQ-1288).
const (
	// SortUpdated orders by updatedAt, newest first; the zero Sort.
	SortUpdated = "updated"
	// SortCreated orders by createdAt, newest first.
	SortCreated = "created"
)

// Filter selects the entries List returns, the narrowing of julieta
// memory list (04 4.3, REQ-1293; 08 8.1, REQ-0653). Query matches an
// entry whose title, body or a tag holds it, ignoring case; Status,
// when set, is the status the entry has; Tags must each be a tag of
// the entry, and an entry without a type tag is never matched by them
// (REQ-1292); Since, when set, is the earliest updatedAt. Walk applies
// these; List also applies Sort, SortUpdated when empty, and Limit, the
// most entries it returns when above zero. The zero Filter selects
// every entry.
type Filter struct {
	Query  string
	Status string
	Tags   []string
	Since  time.Time
	Limit  int
	Sort   string
}

// match reports whether e is selected by f.
func (f Filter) match(e Entry) bool {
	if f.Status != "" && e.Status != f.Status {
		return false
	}
	if len(f.Tags) > 0 && Type(e.Tags) == NoType {
		return false
	}
	for _, t := range f.Tags {
		if !slices.Contains(e.Tags, t) {
			return false
		}
	}
	if !f.Since.IsZero() && e.UpdatedAt.Before(f.Since) {
		return false
	}
	if f.Query == "" {
		return true
	}
	q := strings.ToLower(f.Query)
	if strings.Contains(strings.ToLower(e.Title), q) || strings.Contains(strings.ToLower(e.Body), q) {
		return true
	}
	return slices.ContainsFunc(e.Tags, func(t string) bool { return strings.Contains(t, q) })
}

// List returns the entries f selects, newest first by the time f.Sort
// names, an equal time ordered by id, newest first too, and at most
// f.Limit of them when it is above zero. A Sort other than SortUpdated,
// SortCreated and empty is an error.
func (r *Reader) List(f Filter) ([]Entry, error) {
	at := func(e Entry) time.Time { return e.UpdatedAt }
	switch f.Sort {
	case "", SortUpdated:
	case SortCreated:
		at = func(e Entry) time.Time { return e.CreatedAt }
	default:
		return nil, fmt.Errorf("memstore: the sort \"%s\" is not %s or %s", clip(f.Sort), SortUpdated, SortCreated)
	}
	var out []Entry
	err := r.Walk(f, func(e Entry) bool {
		out = append(out, e)
		return true
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(out, func(a, b Entry) int {
		if c := at(b).Compare(at(a)); c != 0 {
			return c
		}
		return bytes.Compare(b.ID[:], a.ID[:])
	})
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

// Walk calls visit with each entry f selects, oldest first (by id),
// one entry in memory at a time, until visit returns false; f.Sort and
// f.Limit are List's and Walk leaves them. A memory
// dir without entries/ has none. A temporary name of julieta is never
// read (REQ-0654); entries/ that is no directory (a symlink among
// them), any other member of it that is not a file <ULID>.md, and more
// than MaxEntries entries are ErrLayout, and an entry that does not
// decode is its error, which names its file: no entry is skipped.
func (r *Reader) Walk(f Filter, visit func(Entry) bool) error {
	ids, err := r.ids()
	if err != nil {
		return err
	}
	for _, u := range ids {
		e, err := r.get(u)
		if err != nil {
			return err
		}
		if f.match(e) && !visit(e) {
			return nil
		}
	}
	return nil
}

// entriesDir reports whether entries/ exists; one that is not a real
// directory, a symlink to another directory of the memory dir among
// them, is ErrLayout.
func (r *Reader) entriesDir() (bool, error) {
	info, err := r.root.Lstat(EntriesDir)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("%w: %s is not a directory", ErrLayout, EntriesDir)
	}
	return true, nil
}

// ids returns the ids of the entries of entries/, sorted. A dir that
// holds more names than MaxEntries entries and maxTemps temporary
// files is ErrLayout, so no part of it is read silently, and so is one
// with more temporary files than maxTemps.
func (r *Reader) ids() ([]canon.ULID, error) {
	ok, err := r.entriesDir()
	if err != nil || !ok {
		return nil, err
	}
	names, more, err := readNames(r.root, EntriesDir, MaxEntries+maxTemps)
	if err != nil {
		return nil, err
	}
	if more {
		return nil, fmt.Errorf("%w: %s holds more than %d names", ErrLayout, EntriesDir, MaxEntries+maxTemps)
	}
	var ids []canon.ULID
	temps := 0
	for _, name := range names {
		if IsTemp(name) {
			if temps++; temps > maxTemps {
				return nil, fmt.Errorf("%w: %s holds more than %d temporary files", ErrLayout, EntriesDir, maxTemps)
			}
			continue
		}
		u, ok := entryID(name)
		if !ok {
			return nil, fmt.Errorf("%w: %s/%s is not a file <ULID>.md", ErrLayout, EntriesDir, clip(name))
		}
		ids = append(ids, u)
	}
	if len(ids) > MaxEntries {
		return nil, fmt.Errorf("%w: %s holds more than %d entries", ErrLayout, EntriesDir, MaxEntries)
	}
	return ids, nil
}

// Count returns the number of entries of entries/, as List reads them.
func (r *Reader) Count() (int, error) {
	ids, err := r.ids()
	return len(ids), err
}

// maxTemps is how many temporary files entries/ may hold beside its
// entries, so a dir of a few leftover temporary files still lists.
const maxTemps = 1000
