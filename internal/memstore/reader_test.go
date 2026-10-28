// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memstore_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// The ids of the entries the reader tests plant.
const (
	idA = "01j9z3k8m6t2q4w5e7r9y1v3k5"
	idB = "01j9z3k8m6t2q4w5e7r9y1v3k6"
)

// plant writes the entry e to the memory dir dir and returns its path.
func plant(t *testing.T, dir string, e memstore.Entry) string {
	t.Helper()
	data, err := memstore.Encode(e)
	require.NoError(t, err)
	p := filepath.Join(dir, filepath.FromSlash(memstore.EntryPath(e.ID)))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
	require.NoError(t, os.WriteFile(p, data, 0o600))
	return p
}

// open returns the reader of dir, closed when the test ends.
func open(t *testing.T, dir string) *memstore.Reader {
	t.Helper()
	r, err := memstore.Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	return r
}

// two plants the example as idA and a done entry tagged lesson as idB.
func two(t *testing.T) (dir string, a, b memstore.Entry) {
	t.Helper()
	dir = t.TempDir()
	a = example(t)
	b = example(t)
	u, err := canon.ParseULID(idB)
	require.NoError(t, err)
	b.ID, b.Title, b.Status, b.Tags, b.Body = u, "Signing waits on the agent", memstore.StatusDone, []string{"lesson"}, "Ask the operator.\n"
	plant(t, dir, a)
	plant(t, dir, b)
	return dir, a, b
}

// TestListAndGet lists the entries newest first, an equal updatedAt
// ordered by id, filters by query and
// status, and gets one by id; a dir with no entries/ has none, and a
// temporary file is never read (REQ-0654).
func TestListAndGet(t *testing.T) {
	dir, a, b := two(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "entries", ".tmp-01j9z3k8m6t2q4w5e7r9y1v3k7"), []byte("partial"), 0o600))
	r := open(t, dir)
	all, err := r.List(memstore.Filter{})
	require.NoError(t, err)
	assert.Equal(t, []memstore.Entry{b, a}, all)
	for q, want := range map[string][]memstore.Entry{"ARM64": {a}, "operator": {b}, "less": {b}, "nothing": nil, "": {b, a}} {
		got, err := r.List(memstore.Filter{Query: q})
		require.NoError(t, err)
		assert.Equal(t, want, got, q)
	}
	open1, err := r.List(memstore.Filter{Status: memstore.StatusOpen})
	require.NoError(t, err)
	assert.Equal(t, []memstore.Entry{a}, open1)
	got, err := r.Get(idB)
	require.NoError(t, err)
	assert.Equal(t, b, got)
	n, err := r.Count()
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, dir+"/entries/"+idA+".md", r.Path(a.ID))
	assert.Equal(t, dir, r.Dir())

	empty, err := open(t, t.TempDir()).List(memstore.Filter{})
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// TestGetRefuses refuses an id that is no lowercase ULID, ".." among
// them, before it becomes a path, and reports an absent entry.
func TestGetRefuses(t *testing.T) {
	dir, _, _ := two(t)
	r := open(t, dir)
	for _, id := range []string{"..", "../" + idA, idA + "/x", strings.ToUpper(idA), "", "01j9z3k8m6t2q4w5e7r9y1v3k9"} {
		_, err := r.Get(id)
		require.ErrorIs(t, err, memstore.ErrNotFound, id)
	}
}

// TestGetRefusesWhatIsNoRegularFile refuses an entry that is a symlink,
// one to a file outside the dir included, or a FIFO, never following
// or waiting on it (I12 of 05 5.2).
func TestGetRefusesWhatIsNoRegularFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "entries"), 0o700))
	outside := plant(t, t.TempDir(), example(t))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "entries", idA+".md")))
	require.NoError(t, syscall.Mkfifo(filepath.Join(dir, "entries", idB+".md"), 0o600))
	r := open(t, dir)
	for _, id := range []string{idA, idB} {
		_, err := r.Get(id)
		require.ErrorIs(t, err, memstore.ErrLayout, id)
	}
	_, err := r.List(memstore.Filter{})
	require.ErrorIs(t, err, memstore.ErrLayout)
}

// TestListRefuses refuses a dir whose entries/ holds a stray name, is a
// file, or holds an entry that does not decode or whose id is not its
// name: no entry is skipped.
func TestListRefuses(t *testing.T) {
	stray := t.TempDir()
	plant(t, stray, example(t))
	require.NoError(t, os.WriteFile(filepath.Join(stray, "entries", "notes.txt"), nil, 0o600))

	file := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(file, "entries"), nil, 0o600))

	bad := t.TempDir()
	p := plant(t, bad, example(t))
	require.NoError(t, os.WriteFile(p, []byte("not an entry"), 0o600))

	moved := t.TempDir()
	p = plant(t, moved, example(t))
	require.NoError(t, os.Rename(p, filepath.Join(filepath.Dir(p), idB+".md")))

	big := t.TempDir()
	p = plant(t, big, example(t))
	require.NoError(t, os.WriteFile(p, make([]byte, memstore.MaxFileSize+1), 0o600))

	for name, c := range map[string]struct {
		dir  string
		want error
	}{
		"a stray name": {stray, memstore.ErrLayout}, "a file": {file, memstore.ErrLayout},
		"a bad entry": {bad, memstore.ErrInvalid}, "a moved entry": {moved, memstore.ErrInvalid},
		"a file over the cap": {big, memstore.ErrInvalid},
	} {
		_, err := open(t, c.dir).List(memstore.Filter{})
		require.ErrorIs(t, err, c.want, name)
	}
}

// TestOpenRefuses refuses a memory dir that is absent, a file, a
// symlink, or no absolute and clean path.
func TestOpenRefuses(t *testing.T) {
	base := t.TempDir()
	file := filepath.Join(base, "f")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	link := filepath.Join(base, "l")
	require.NoError(t, os.Symlink(t.TempDir(), link))
	for _, p := range []string{filepath.Join(base, "absent"), file, link, "rel", base + "/./x"} {
		_, err := memstore.Open(p)
		require.Error(t, err, p)
	}
}

// TestEntriesSymlink refuses an entries/ that is a symlink to another
// directory of the memory dir, on Get as on List.
func TestEntriesSymlink(t *testing.T) {
	dir := t.TempDir()
	plant(t, filepath.Join(dir, "salvage"), example(t))
	require.NoError(t, os.Symlink(filepath.Join(dir, "salvage", "entries"), filepath.Join(dir, "entries")))
	r := open(t, dir)
	_, err := r.Get(idA)
	require.ErrorIs(t, err, memstore.ErrLayout)
	_, err = r.List(memstore.Filter{})
	require.ErrorIs(t, err, memstore.ErrLayout)
}

// TestListRefusesTooManyTemps refuses an entries/ with more temporary
// files than the reader allows, rather than reading part of it.
func TestListRefusesTooManyTemps(t *testing.T) {
	dir := t.TempDir()
	plant(t, dir, example(t))
	src := canon.NewULIDSource(bytes.NewReader(make([]byte, 10)))
	for range 1001 {
		u, err := canon.NewULID(example(t).CreatedAt, src)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "entries", memstore.TempName(u)), nil, 0o600))
	}
	_, err := open(t, dir).List(memstore.Filter{})
	require.ErrorIs(t, err, memstore.ErrLayout)
	assert.Contains(t, err.Error(), "temporary files")
}

// TestErrorsEscapeNames escapes a stray name that reaches an error, so
// it cannot forge a line of the output.
func TestErrorsEscapeNames(t *testing.T) {
	dir := t.TempDir()
	plant(t, dir, example(t))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "entries", "x\nfinding\x1b[0m"), nil, 0o600))
	_, err := open(t, dir).List(memstore.Filter{})
	require.ErrorIs(t, err, memstore.ErrLayout)
	assert.NotContains(t, err.Error(), "\n")
	assert.NotContains(t, err.Error(), "\x1b")
}

// TestListFilters runs each narrowing of julieta memory list alone and
// all together (04 4.3, REQ-1293; REQ-0653), on three entries whose
// updatedAt and createdAt orders differ: every given tag must hold, an
// entry without a type tag is found by its free tags (REQ-1292), since
// is the earliest updatedAt, the sort is updatedAt or createdAt newest
// first (REQ-1288), and the limit cuts after the sort.
func TestListFilters(t *testing.T) {
	dir := t.TempDir()
	day := func(d, h int) time.Time { return time.Date(2026, 10, d, h, 0, 0, 0, time.UTC) }
	mk := func(id, title, status string, tags []string, created, updated time.Time) memstore.Entry {
		e := example(t)
		u, err := canon.ParseULID(id)
		require.NoError(t, err)
		e.ID, e.Title, e.Status, e.Tags, e.CreatedAt, e.UpdatedAt = u, title, status, tags, created, updated
		plant(t, dir, e)
		return e
	}
	fb := mk(idA, "feedback on ci", memstore.StatusOpen, []string{"ci", "feedback"}, day(3, 0), day(27, 12))
	pr := mk(idB, "a project note", memstore.StatusDone, []string{"project"}, day(1, 0), day(28, 9))
	old := mk("01j9z3k8m6t2q4w5e7r9y1v3k8", "an entry of before the rule", memstore.StatusOpen, []string{"ci"}, day(2, 0), day(28, 11))
	r := open(t, dir)
	for name, c := range map[string]struct {
		f    memstore.Filter
		want []memstore.Entry
	}{
		"none":                   {memstore.Filter{}, []memstore.Entry{old, pr, fb}},
		"sort updated":           {memstore.Filter{Sort: memstore.SortUpdated}, []memstore.Entry{old, pr, fb}},
		"sort created":           {memstore.Filter{Sort: memstore.SortCreated}, []memstore.Entry{fb, old, pr}},
		"a free tag":             {memstore.Filter{Tags: []string{"ci"}}, []memstore.Entry{old, fb}},
		"a type tag":             {memstore.Filter{Tags: []string{"project"}}, []memstore.Entry{pr}},
		"two tags that hold":     {memstore.Filter{Tags: []string{"ci", "feedback"}}, []memstore.Entry{fb}},
		"two tags, one missing":  {memstore.Filter{Tags: []string{"feedback", "project"}}, nil},
		"since a date":           {memstore.Filter{Since: day(28, 0)}, []memstore.Entry{old, pr}},
		"since a timestamp":      {memstore.Filter{Since: day(28, 10)}, []memstore.Entry{old}},
		"since exactly updated":  {memstore.Filter{Since: day(28, 9)}, []memstore.Entry{old, pr}},
		"limit":                  {memstore.Filter{Limit: 1}, []memstore.Entry{old}},
		"limit over the entries": {memstore.Filter{Limit: 9}, []memstore.Entry{old, pr, fb}},
		"status":                 {memstore.Filter{Status: memstore.StatusOpen}, []memstore.Entry{old, fb}},
		"query":                  {memstore.Filter{Query: "NOTE"}, []memstore.Entry{pr}},
		"all together": {memstore.Filter{Query: "o", Status: memstore.StatusOpen, Tags: []string{"feedback"},
			Since: day(27, 0), Limit: 1, Sort: memstore.SortCreated}, []memstore.Entry{fb}},
	} {
		got, err := r.List(c.f)
		require.NoError(t, err, name)
		assert.Equal(t, c.want, got, name)
	}
	_, err := r.List(memstore.Filter{Sort: "title"})
	require.Error(t, err)
}
