// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memstore

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"
	"syscall"

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
		return Entry{}, fmt.Errorf("%w: the id %q: %w", ErrNotFound, clip(id), err)
	}
	return r.get(u)
}

// get is Get for a parsed id.
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
	f, err := r.root.OpenFile(p, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
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

// Filter selects the entries List returns: Query, the text of julieta
// memory list --query, matches an entry whose title, body or a tag
// holds it, ignoring case; Status, when set, is the status the entry
// has. The zero Filter selects every entry.
type Filter struct {
	Query  string
	Status string
}

// match reports whether e is selected by f.
func (f Filter) match(e Entry) bool {
	if f.Status != "" && e.Status != f.Status {
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

// List returns the entries f selects, oldest first (by id). A memory
// dir without entries/ has none. A temporary name of julieta is never
// read (REQ-0654); any other member of entries/ that is not a file
// <ULID>.md, and more than MaxEntries entries, are ErrLayout, and an
// entry that does not decode is its error: no entry is skipped.
func (r *Reader) List(f Filter) ([]Entry, error) {
	ids, err := r.ids()
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, u := range ids {
		e, err := r.get(u)
		if err != nil {
			return nil, err
		}
		if f.match(e) {
			out = append(out, e)
		}
	}
	return out, nil
}

// ids returns the ids of the entries of entries/, sorted.
func (r *Reader) ids() ([]canon.ULID, error) {
	info, err := r.root.Lstat(EntriesDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s is not a directory", ErrLayout, EntriesDir)
	}
	names, err := readNames(r.root, EntriesDir, MaxEntries+maxTemps+1)
	if err != nil {
		return nil, err
	}
	var ids []canon.ULID
	for _, name := range names {
		if IsTemp(name) {
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

// maxTemps is how many temporary names past MaxEntries the reader of
// entries/ reads before it stops, so a dir of leftover temporary files
// still lists.
const maxTemps = 1000
