// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memstore_test

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

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

// TestListAndGet lists the entries oldest first, filters by query and
// status, and gets one by id; a dir with no entries/ has none, and a
// temporary file is never read (REQ-0654).
func TestListAndGet(t *testing.T) {
	dir, a, b := two(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "entries", ".tmp-01j9z3k8m6t2q4w5e7r9y1v3k7"), []byte("partial"), 0o600))
	r := open(t, dir)
	all, err := r.List(memstore.Filter{})
	require.NoError(t, err)
	assert.Equal(t, []memstore.Entry{a, b}, all)
	for q, want := range map[string][]memstore.Entry{"ARM64": {a}, "operator": {b}, "less": {b}, "nothing": nil, "": {a, b}} {
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
