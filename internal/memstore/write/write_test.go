// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package write

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// clock is the time the tests stamp with (10 10.4).
var clock = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

// entropy is the fixed entropy of the tests' ULIDs.
func entropy() *canon.ULIDSource {
	return canon.NewULIDSource(bytes.NewReader(bytes.Repeat([]byte{7}, 1<<16)))
}

// newStore returns a store on a new memory dir, with its lock below a
// new HOME, and the clock it reads, which the test may move.
func newStore(t *testing.T) (*Store, *time.Time) {
	t.Helper()
	return storeAt(t, t.TempDir(), t.TempDir())
}

// storeAt returns the store of the memory dir dir with the lock of the
// repo api below home.
func storeAt(t *testing.T, dir, home string) (*Store, *time.Time) {
	t.Helper()
	now := clock
	s, err := Open(Config{Dir: dir, Lock: LockPath(home, "api"), Now: func() time.Time { return now }, ULIDs: entropy()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s, &now
}

// tree returns every path below dir with its mode, slash-separated.
func tree(t *testing.T, dir string) map[string]fs.FileMode {
	t.Helper()
	out := map[string]fs.FileMode{}
	require.NoError(t, filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		require.NoError(t, err)
		if p == dir {
			return nil
		}
		info, err := d.Info()
		require.NoError(t, err)
		rel, err := filepath.Rel(dir, p)
		require.NoError(t, err)
		out[filepath.ToSlash(rel)] = info.Mode()
		return nil
	}))
	return out
}

// TestAddStamps adds an entry: julieta stamps the id from the clock and
// the injected source and both times (REQ-0656), the file is the one
// entries/<id>.md, 0600 in a dir 0700 (REQ-0646), and it reads back.
func TestAddStamps(t *testing.T) {
	s, _ := newStore(t)
	e, wrote, err := s.Add(memstore.Content{Title: "Flaky test", Body: "On arm64.\n", Tags: []string{"ci", "arm64", "ci"}})
	require.NoError(t, err)
	assert.True(t, wrote)
	want, err := canon.NewULID(clock, entropy())
	require.NoError(t, err)
	assert.Equal(t, want, e.ID)
	assert.Equal(t, clock, e.CreatedAt)
	assert.Equal(t, clock, e.UpdatedAt)
	assert.Equal(t, memstore.StatusOpen, e.Status)
	assert.Equal(t, []string{"arm64", "ci"}, e.Tags)
	assert.Equal(t, map[string]fs.FileMode{
		"entries":                          fs.ModeDir | 0o700,
		"entries/" + e.ID.String() + ".md": 0o600,
	}, tree(t, s.Dir()))
	got, err := s.Get(e.ID.String())
	require.NoError(t, err)
	assert.Equal(t, e, got)
}

// TestAddDuplicate adds nothing for content whose memory-content digest
// equals an open entry's and returns that entry (04 4.1, Idempotency);
// the same content as a done entry is a new entry.
func TestAddDuplicate(t *testing.T) {
	s, now := newStore(t)
	c := memstore.Content{Title: "t", Body: "b", Tags: []string{"x"}}
	first, wrote, err := s.Add(c)
	require.NoError(t, err)
	require.True(t, wrote)
	*now = now.Add(time.Hour)
	again, wrote, err := s.Add(memstore.Content{Title: "t", Body: "b", Tags: []string{"x", "x"}, Status: memstore.StatusOpen})
	require.NoError(t, err)
	assert.False(t, wrote)
	assert.Equal(t, first, again)
	assert.Len(t, tree(t, s.Dir()), 2)

	done := memstore.StatusDone
	_, _, err = s.Edit(first.ID.String(), Change{Status: &done})
	require.NoError(t, err)
	third, wrote, err := s.Add(c)
	require.NoError(t, err)
	assert.True(t, wrote)
	assert.NotEqual(t, first.ID, third.ID)
}

// TestAddStopsAtTheFirstMatch stops the duplicate walk at the open
// entry that matches: an entry after it that does not decode is not
// read, while new content, whose walk reads every entry, fails on it,
// naming its file.
func TestAddStopsAtTheFirstMatch(t *testing.T) {
	s, _ := newStore(t)
	c := memstore.Content{Title: "t"}
	first, _, err := s.Add(c)
	require.NoError(t, err)
	later, err := canon.NewULID(clock.Add(time.Hour), entropy())
	require.NoError(t, err)
	bad := filepath.Join(s.Dir(), filepath.FromSlash(memstore.EntryPath(later)))
	require.NoError(t, os.WriteFile(bad, []byte("not an entry"), 0o600))
	again, wrote, err := s.Add(c)
	require.NoError(t, err)
	assert.False(t, wrote)
	assert.Equal(t, first, again)
	_, _, err = s.Add(memstore.Content{Title: "new"})
	require.ErrorIs(t, err, memstore.ErrInvalid)
	assert.Contains(t, err.Error(), memstore.EntryPath(later))
}

// TestAddRefuses refuses content that breaks 03 3.9, and an add to a
// full store.
func TestAddRefuses(t *testing.T) {
	s, _ := newStore(t)
	for _, c := range []memstore.Content{{}, {Title: "t", Status: "closed"}, {Title: "t", Tags: []string{"A"}}} {
		_, _, err := s.Add(c)
		require.ErrorIs(t, err, memstore.ErrInvalid)
	}
	assert.Empty(t, tree(t, s.Dir()))
}

// TestEdit closes an entry with --status done and stamps updatedAt
// only; an edit that changes nothing writes nothing (04 4.1,
// Idempotency).
func TestEdit(t *testing.T) {
	s, now := newStore(t)
	e, _, err := s.Add(memstore.Content{Title: "t", Body: "b"})
	require.NoError(t, err)
	*now = now.Add(90 * time.Minute)
	done, title, tags, body := memstore.StatusDone, "t2", []string{"lesson"}, "b2"
	got, wrote, err := s.Edit(e.ID.String(), Change{Status: &done, Title: &title, Tags: &tags, Body: &body})
	require.NoError(t, err)
	assert.True(t, wrote)
	assert.Equal(t, e.ID, got.ID)
	assert.Equal(t, clock, got.CreatedAt)
	assert.Equal(t, clock.Add(90*time.Minute), got.UpdatedAt)
	assert.Equal(t, memstore.Content{Title: "t2", Body: "b2", Status: done, Tags: []string{"lesson"}}, got.Content())
	read, err := s.Get(e.ID.String())
	require.NoError(t, err)
	assert.Equal(t, got, read)

	*now = now.Add(time.Hour)
	same, wrote, err := s.Edit(e.ID.String(), Change{Status: &done})
	require.NoError(t, err)
	assert.False(t, wrote)
	assert.Equal(t, got, same)

	*now = clock.Add(-time.Hour)
	open := memstore.StatusOpen
	back, _, err := s.Edit(e.ID.String(), Change{Status: &open})
	require.NoError(t, err)
	assert.Equal(t, clock, back.UpdatedAt, "a clock behind createdAt keeps the times in order")
}

// TestEditRefuses refuses an absent id, an id that is no ULID, and a
// change that breaks 03 3.9.
func TestEditRefuses(t *testing.T) {
	s, _ := newStore(t)
	e, _, err := s.Add(memstore.Content{Title: "t"})
	require.NoError(t, err)
	_, _, err = s.Edit("01j9z3k8m6t2q4w5e7r9y1v3k5", Change{})
	require.ErrorIs(t, err, memstore.ErrNotFound)
	_, _, err = s.Edit("..", Change{})
	require.ErrorIs(t, err, memstore.ErrNotFound)
	bad := "closed"
	_, _, err = s.Edit(e.ID.String(), Change{Status: &bad})
	require.ErrorIs(t, err, memstore.ErrInvalid)
}

// TestLockIsOutsideTheMemoryDir holds the store lock at
// $HOME/.local/state/julieta/locks/<dir>.lock, 0600 in a dir 0700, and
// no file appears in the memory dir while it is held (REQ-0655).
func TestLockIsOutsideTheMemoryDir(t *testing.T) {
	home := t.TempDir()
	dir := t.TempDir()
	s, _ := storeAt(t, dir, home)
	lock := filepath.Join(home, ".local", "state", "julieta", "locks", "api.lock")
	require.Equal(t, lock, s.lock)
	require.NoError(t, s.locked(func() error {
		assert.Empty(t, tree(t, dir), "no file in the memory dir while the lock is held")
		info, err := os.Stat(lock)
		require.NoError(t, err)
		assert.Equal(t, fs.FileMode(0o600), info.Mode())
		info, err = os.Stat(filepath.Dir(lock))
		require.NoError(t, err)
		assert.Equal(t, fs.FileMode(0o700), info.Mode().Perm())
		return nil
	}))
}

// TestMoveAStore copies entries/ whole to a new memory dir in a new
// HOME, with no lock file: the copy is a complete move, and the store
// lock is recreated on first use (REQ-0647, REQ-0243).
func TestMoveAStore(t *testing.T) {
	s, _ := newStore(t)
	a, _, err := s.Add(memstore.Content{Title: "a", Body: "one"})
	require.NoError(t, err)
	b, _, err := s.Add(memstore.Content{Title: "b", Tags: []string{"lesson"}})
	require.NoError(t, err)

	dir, home := t.TempDir(), t.TempDir()
	require.NoError(t, os.CopyFS(filepath.Join(dir, "entries"), os.DirFS(filepath.Join(s.Dir(), "entries"))))
	moved, _ := storeAt(t, dir, home)
	all, err := moved.List(memstore.Filter{})
	require.NoError(t, err)
	assert.Equal(t, []memstore.Entry{a, b}, all)
	_, err = os.Stat(moved.lock)
	require.ErrorIs(t, err, fs.ErrNotExist)
	_, wrote, err := moved.Add(memstore.Content{Title: "c"})
	require.NoError(t, err)
	assert.True(t, wrote)
	_, err = os.Stat(moved.lock)
	require.NoError(t, err)
}

// TestConcurrentWriters runs writers on one store, each with a store of
// its own, so each takes the flock on an open file of its own as a
// process does, and loses no entry (10 10.1, Concurrency); the
// container e2e runs two julieta processes.
func TestConcurrentWriters(t *testing.T) {
	dir, home := t.TempDir(), t.TempDir()
	src := entropy()
	const writers, each = 4, 25
	var wg sync.WaitGroup
	for w := range writers {
		s, err := Open(Config{Dir: dir, Lock: LockPath(home, "api"), Now: func() time.Time { return clock }, ULIDs: src})
		require.NoError(t, err)
		t.Cleanup(func() { _ = s.Close() })
		wg.Go(func() {
			for i := range each {
				_, wrote, err := s.Add(memstore.Content{Title: "w" + string(rune('a'+w)) + strings.Repeat("i", i+1)})
				if err != nil {
					t.Error(err)
					return
				}
				assert.True(t, wrote)
			}
		})
	}
	wg.Wait()
	r, err := memstore.Open(dir)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()
	n, err := r.Count()
	require.NoError(t, err)
	assert.Equal(t, writers*each, n)
}

// TestConcurrentDuplicates runs writers that add the same content at
// once, each with a store of its own: the store lock makes the
// duplicate rule hold across them, so exactly one writes and the
// others get its entry (04 4.1, Idempotency; REQ-0655).
func TestConcurrentDuplicates(t *testing.T) {
	dir, home := t.TempDir(), t.TempDir()
	src := entropy()
	const writers = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	wrote := 0
	ids := map[canon.ULID]bool{}
	start := make(chan struct{})
	for range writers {
		s, err := Open(Config{Dir: dir, Lock: LockPath(home, "api"), Now: func() time.Time { return clock }, ULIDs: src})
		require.NoError(t, err)
		t.Cleanup(func() { _ = s.Close() })
		wg.Go(func() {
			<-start
			e, w, err := s.Add(memstore.Content{Title: "same", Body: strings.Repeat("b", 1<<16)})
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			ids[e.ID] = true
			if w {
				wrote++
			}
		})
	}
	close(start)
	wg.Wait()
	assert.Equal(t, 1, wrote, "one writer wrote")
	assert.Len(t, ids, 1, "every writer got the one entry")
	names, err := os.ReadDir(filepath.Join(dir, "entries"))
	require.NoError(t, err)
	assert.Len(t, names, 1, "one file")
}

// TestRemoveTemps removes a temporary file older than the start and
// keeps a newer one (REQ-0654); one whose ULID is after now no julieta
// wrote yet, and it goes too; a non-temporary dotfile is not its to
// remove.
func TestRemoveTemps(t *testing.T) {
	dir := t.TempDir()
	src := entropy()
	old, err := canon.NewULID(clock.Add(-time.Minute), src)
	require.NoError(t, err)
	newer, err := canon.NewULID(clock.Add(time.Minute), src)
	require.NoError(t, err)
	future, err := canon.NewULID(clock.Add(time.Hour), src)
	require.NoError(t, err)
	for _, p := range []string{"entries/" + memstore.TempName(old), "handoff/" + memstore.TempName(newer), "entries/.hidden", "salvage/" + memstore.TempName(future)} {
		full := filepath.Join(dir, filepath.FromSlash(p))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o700))
		require.NoError(t, os.WriteFile(full, nil, 0o600))
	}
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	defer func() { _ = root.Close() }()
	removed, kept, err := RemoveTemps(root, clock, clock.Add(2*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, []string{"entries/" + memstore.TempName(old), "salvage/" + memstore.TempName(future)}, removed)
	assert.Equal(t, []string{"handoff/" + memstore.TempName(newer)}, kept)
	paths := slices.Sorted(func(yield func(string) bool) {
		for p := range tree(t, dir) {
			if !yield(p) {
				return
			}
		}
	})
	assert.Equal(t, []string{"entries", "entries/.hidden", "handoff", "handoff/" + memstore.TempName(newer), "salvage"}, paths)
}

// TestOpenRefuses refuses a config without a clock or a ULID source, a
// lock that is not an absolute, clean path, and an absent memory dir.
func TestOpenRefuses(t *testing.T) {
	dir := t.TempDir()
	now := func() time.Time { return clock }
	for _, c := range []Config{
		{Dir: dir, Lock: "/l", ULIDs: entropy()},
		{Dir: dir, Lock: "/l", Now: now},
		{Dir: dir, Lock: "l", Now: now, ULIDs: entropy()},
		{Dir: filepath.Join(dir, "absent"), Lock: "/l", Now: now, ULIDs: entropy()},
	} {
		_, err := Open(c)
		require.Error(t, err)
	}
}

// TestPutRefusesAnEntriesFile refuses to write when entries is a file.
func TestPutRefusesAnEntriesFile(t *testing.T) {
	s, _ := newStore(t)
	require.NoError(t, os.WriteFile(filepath.Join(s.Dir(), "entries"), nil, 0o600))
	_, _, err := s.Add(memstore.Content{Title: "t"})
	require.ErrorIs(t, err, memstore.ErrLayout)
}
