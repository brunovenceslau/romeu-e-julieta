// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package actions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore/write"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

// seam sets the test seam *v to value and restores it after the test;
// a test that sets one does not run in parallel.
func seam[T any](t *testing.T, v *T, value T) {
	t.Helper()
	old := *v
	*v = value
	t.Cleanup(func() { *v = old })
}

// TestSnapshotWritesTheBundleThenHeads (08 8.4, REQ-0688, REQ-0689):
// the bundle holds the refs beyond the base, needs the base, and is
// renamed into place before heads.json, which lists every ref the
// snapshot read with the time of the clock; both files are 0600 in a
// snapshot dir 0700, no temporary file remains, and a second snapshot
// replaces the pair, so one bundle is kept.
func TestSnapshotWritesTheBundleThenHeads(t *testing.T) {
	f := newFixture(t)
	c1 := commit(t, f.repo, "one")
	git(t, f.repo, "tag", "at-base", f.base)

	res, err := Snapshot(t.Context(), f.config(t, clock))
	require.NoError(t, err)
	assert.True(t, res.Bundle)
	assert.Equal(t, filepath.Join(f.snapshotDir(), salvage.HeadsFile), res.Path)
	h := f.heads(t)
	assert.Equal(t, clock, h.CreatedAt)
	assert.False(t, h.Disabled)
	assert.Equal(t, map[string]string{"refs/heads/main": c1, "refs/tags/at-base": f.base}, h.Refs)
	assert.Equal(t, res.Heads, h)
	assert.Equal(t, map[string]string{"refs/heads/main": c1}, f.bundleHeads(t), "a ref the base reaches is not in the bundle")
	assert.Contains(t, git(t, f.repo, "bundle", "verify", filepath.Join(f.snapshotDir(), salvage.BundleFile)), f.base,
		"the bundle needs the base")
	for _, p := range []string{f.snapshotDir(), filepath.Join(f.snapshotDir(), salvage.BundleFile), res.Path} {
		info, err := os.Stat(p)
		require.NoError(t, err)
		want := os.FileMode(0o600)
		if info.IsDir() {
			want = 0o700
		}
		assert.Equal(t, want, info.Mode().Perm(), p)
	}
	assert.Equal(t, []string{salvage.BundleFile, salvage.HeadsFile}, names(t, f.snapshotDir()))

	c2 := commit(t, f.repo, "two")
	later := clock.Add(time.Minute)
	_, err = Snapshot(t.Context(), f.config(t, later))
	require.NoError(t, err)
	assert.Equal(t, later, f.heads(t).CreatedAt)
	assert.Equal(t, map[string]string{"refs/heads/main": c2}, f.bundleHeads(t))
	assert.Equal(t, []string{salvage.BundleFile, salvage.HeadsFile}, names(t, f.snapshotDir()), "one bundle is kept")
	assert.Empty(t, names(t, f.config(t, later).Work), "the bundle is built in julieta state and removed there")
}

// TestSnapshotWithNothingBeyondTheBaseHasNoBundle: when the base
// reaches every ref, git refuses an empty bundle; the snapshot removes
// the bundle of an earlier one and writes heads.json, so the pair is
// still this snapshot's.
func TestSnapshotWithNothingBeyondTheBaseHasNoBundle(t *testing.T) {
	f := newFixture(t)
	commit(t, f.repo, "one")
	_, err := Snapshot(t.Context(), f.config(t, clock))
	require.NoError(t, err)
	git(t, f.repo, "reset", "-q", "--hard", f.base)
	git(t, f.repo, "reflog", "expire", "--expire=now", "--all")

	res, err := Snapshot(t.Context(), f.config(t, clock.Add(time.Minute)))
	require.NoError(t, err)
	assert.False(t, res.Bundle)
	assert.Equal(t, []string{salvage.HeadsFile}, names(t, f.snapshotDir()))
	assert.Equal(t, map[string]string{"refs/heads/main": f.base}, f.heads(t).Refs)
}

// TestSnapshotOfARepositoryWithNoRef writes heads.json with no ref and
// no bundle.
func TestSnapshotOfARepositoryWithNoRef(t *testing.T) {
	f := newFixture(t)
	git(t, filepath.Dir(f.repo), "init", "-q", "empty")
	c := f.config(t, clock)
	c.Dir, c.Bases = filepath.Join(filepath.Dir(f.repo), "empty"), nil
	res, err := Snapshot(t.Context(), c)
	require.NoError(t, err)
	assert.False(t, res.Bundle)
	assert.Empty(t, f.heads(t).Refs)
}

// TestSnapshotKilledBetweenTheWrites: a kill after the bundle's rename
// and before heads.json leaves the previous heads.json and a valid
// bundle (REQ-0689); the kill is the seam returning an error.
func TestSnapshotKilledBetweenTheWrites(t *testing.T) {
	f := newFixture(t)
	commit(t, f.repo, "one")
	_, err := Snapshot(t.Context(), f.config(t, clock))
	require.NoError(t, err)
	before, err := os.ReadFile(filepath.Join(f.snapshotDir(), salvage.HeadsFile))
	require.NoError(t, err)

	c2 := commit(t, f.repo, "two")
	killed := errors.New("killed")
	seam(t, &snapshotBetweenWrites, func() error { return killed })
	_, err = Snapshot(t.Context(), f.config(t, clock.Add(time.Minute)))
	require.ErrorIs(t, err, killed)
	after, err := os.ReadFile(filepath.Join(f.snapshotDir(), salvage.HeadsFile))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "the previous heads.json")
	assert.Equal(t, map[string]string{"refs/heads/main": c2}, f.bundleHeads(t), "a valid bundle, the new one")
}

// TestSnapshotWriteFailureKeepsThePreviousBundle (REQ-0691): a write of
// the bundle that fails keeps the previous bundle and heads.json and
// leaves no temporary file of this run.
func TestSnapshotWriteFailureKeepsThePreviousBundle(t *testing.T) {
	f := newFixture(t)
	c1 := commit(t, f.repo, "one")
	_, err := Snapshot(t.Context(), f.config(t, clock))
	require.NoError(t, err)
	bundle, err := os.ReadFile(filepath.Join(f.snapshotDir(), salvage.BundleFile))
	require.NoError(t, err)

	commit(t, f.repo, "two")
	full := errors.New("no space left on device")
	seam(t, &snapshotSync, func(*os.File) error { return full })
	_, err = Snapshot(t.Context(), f.config(t, clock.Add(time.Minute)))
	require.ErrorIs(t, err, full)
	now, err := os.ReadFile(filepath.Join(f.snapshotDir(), salvage.BundleFile))
	require.NoError(t, err)
	assert.Equal(t, bundle, now, "the previous bundle stays")
	assert.Equal(t, map[string]string{"refs/heads/main": c1}, f.heads(t).Refs, "and the previous heads.json")
	assert.Equal(t, []string{salvage.BundleFile, salvage.HeadsFile}, names(t, f.snapshotDir()), "no .tmp- file remains")
}

// TestSnapshotRacingACommitLeavesAValidBundle (10 10.1, REQ-0778): a
// commit and an amend land between the read of the refs and the bundle,
// forced by the seam; the bundle is valid and holds the refs read, and
// heads.json lists the same.
func TestSnapshotRacingACommitLeavesAValidBundle(t *testing.T) {
	f := newFixture(t)
	c1 := commit(t, f.repo, "one")
	seam(t, &snapshotAfterRead, func() {
		commit(t, f.repo, "two")
		git(t, f.repo, "commit", "-q", "--amend", "--allow-empty", "-m", "two amended")
	})
	_, err := Snapshot(t.Context(), f.config(t, clock))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"refs/heads/main": c1}, f.bundleHeads(t))
	assert.Equal(t, map[string]string{"refs/heads/main": c1}, f.heads(t).Refs)
}

// TestTwoSnapshotsAreSerializedByTheStoreLock (08 8.4, REQ-0690; 10
// 10.1, REQ-0779): a second snapshot of the repo waits for the store
// lock the first holds from before its read to after its last write,
// so the pair on disk is one snapshot's, the second's. The interleaving
// is forced by the seams, never by a sleep: the first snapshot holds
// the lock until the second reports its wait, and a commit lands in
// between, so the two snapshots differ.
func TestTwoSnapshotsAreSerializedByTheStoreLock(t *testing.T) {
	f := newFixture(t)
	commit(t, f.repo, "one")
	secondConfig := f.config(t, clock.Add(time.Minute))
	waiting := make(chan struct{})
	seam(t, &snapshotOnWait, func() { close(waiting) })
	second := make(chan error, 1)
	calls := 0
	var c2 string
	seam(t, &snapshotAfterRead, func() {
		calls++
		if calls > 1 {
			return
		}
		go func() {
			_, err := Snapshot(t.Context(), secondConfig)
			second <- err
		}()
		select {
		case <-waiting:
		case <-time.After(30 * time.Second):
			t.Fatal("the second snapshot never waited for the lock")
		}
		c2 = commit(t, f.repo, "two")
	})
	_, err := Snapshot(t.Context(), f.config(t, clock))
	require.NoError(t, err)
	select {
	case err := <-second:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("the second snapshot never finished")
	}
	h := f.heads(t)
	assert.Equal(t, clock.Add(time.Minute), h.CreatedAt, "the second snapshot wrote last")
	assert.Equal(t, map[string]string{"refs/heads/main": c2}, h.Refs)
	assert.Equal(t, h.Refs, f.bundleHeads(t), "the pair on disk is one snapshot's")
}

// TestSnapshotRemovesWhatAKilledSnapshotLeft (decided while building
// T060): a killed snapshot can leave gitsafe's temporary repository
// (.gitsafe-bundle-*) and its bundle in the snapshot's own directory of
// julieta state; the next snapshot of the repo, under the store lock,
// removes both.
func TestSnapshotRemovesWhatAKilledSnapshotLeft(t *testing.T) {
	f := newFixture(t)
	commit(t, f.repo, "one")
	c := f.config(t, clock)
	require.NoError(t, os.MkdirAll(filepath.Join(c.Work, ".gitsafe-bundle-123", "objects"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(c.Work, "heads-01j9zc2q8x7w4t6y3r5e1v9k0p.bundle"), []byte("x"), 0o600))
	_, err := Snapshot(t.Context(), c)
	require.NoError(t, err)
	assert.Empty(t, names(t, c.Work))
}

// TestSnapshotRecordsARepoLocalHooksPath (REQ-0692): a repo whose own
// config sets core.hooksPath bypasses the dispatcher, and heads.json
// says disabled.
func TestSnapshotRecordsARepoLocalHooksPath(t *testing.T) {
	f := newFixture(t)
	commit(t, f.repo, "one")
	git(t, f.repo, "config", "core.hooksPath", ".husky")
	_, err := Snapshot(t.Context(), f.config(t, clock))
	require.NoError(t, err)
	assert.True(t, f.heads(t).Disabled)
}

// TestSnapshotOfASkippedInputIsWrittenAndFails (decided while building
// T060): an input gitsafe refuses whole is left out, the rest is
// written, and the error names it, so the dispatcher records a hook
// failure with the repo.
func TestSnapshotOfASkippedInputIsWrittenAndFails(t *testing.T) {
	f := newFixture(t)
	c1 := commit(t, f.repo, "one")
	git(t, f.repo, "update-ref", "refs/tags/bad\u202etag", c1)
	res, err := Snapshot(t.Context(), f.config(t, clock))
	require.ErrorIs(t, err, ErrSnapshotPartial)
	assert.Contains(t, err.Error(), "refs/tags/")
	assert.Equal(t, map[string]string{"refs/heads/main": c1}, f.heads(t).Refs)
	assert.Len(t, res.Skipped, 1)
}

// TestSnapshotRefusesWhatItCannotWrite: a memory dir that is not an
// absolute, clean path, a snapshot path that is no directory, a work
// directory below a file, a directory that is no repository and a
// bundle path that cannot be removed each fail the snapshot before or
// in place of its writes.
func TestSnapshotRefusesWhatItCannotWrite(t *testing.T) {
	f := newFixture(t)
	commit(t, f.repo, "one")

	c := f.config(t, clock)
	c.Memory = "memory"
	_, err := Snapshot(t.Context(), c)
	require.ErrorContains(t, err, "not an absolute, clean path")

	file := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	c = f.config(t, clock)
	c.Work = filepath.Join(file, "work")
	_, err = Snapshot(t.Context(), c)
	require.Error(t, err)

	c = f.config(t, clock)
	c.Dir = t.TempDir()
	_, err = Snapshot(t.Context(), c)
	require.Error(t, err, "no repository")

	require.NoError(t, os.RemoveAll(f.snapshotDir()))
	require.NoError(t, os.WriteFile(f.snapshotDir(), nil, 0o600))
	_, err = Snapshot(t.Context(), f.config(t, clock))
	require.ErrorIs(t, err, memstore.ErrLayout)
	require.NoError(t, os.Remove(f.snapshotDir()))

	// Nothing beyond the base, and a bundle path that is a directory
	// with a file in it, which no remove takes.
	git(t, f.repo, "reset", "-q", "--hard", f.base)
	git(t, f.repo, "reflog", "expire", "--expire=now", "--all")
	require.NoError(t, os.MkdirAll(filepath.Join(f.snapshotDir(), salvage.BundleFile, "x"), 0o700))
	_, err = Snapshot(t.Context(), f.config(t, clock))
	require.Error(t, err)
	_, err = os.Stat(filepath.Join(f.snapshotDir(), salvage.HeadsFile))
	require.ErrorIs(t, err, os.ErrNotExist, "no heads.json after a failed first write")
}

// TestAPartialSnapshotNeverRemovesTheBundle (decided while building
// T060): a plan that left an input out and holds nothing beyond the
// base writes nothing, so the previous pair stays, and fails.
func TestAPartialSnapshotNeverRemovesTheBundle(t *testing.T) {
	f := newFixture(t)
	commit(t, f.repo, "one")
	_, err := Snapshot(t.Context(), f.config(t, clock))
	require.NoError(t, err)
	before, err := os.ReadFile(filepath.Join(f.snapshotDir(), salvage.HeadsFile))
	require.NoError(t, err)

	git(t, f.repo, "reset", "-q", "--hard", f.base)
	git(t, f.repo, "reflog", "expire", "--expire=now", "--all")
	git(t, f.repo, "update-ref", "refs/tags/bad\u202etag", f.base)
	_, err = Snapshot(t.Context(), f.config(t, clock.Add(time.Minute)))
	require.ErrorIs(t, err, ErrSnapshotPartial)
	assert.Equal(t, []string{salvage.BundleFile, salvage.HeadsFile}, names(t, f.snapshotDir()))
	after, err := os.ReadFile(filepath.Join(f.snapshotDir(), salvage.HeadsFile))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after))
}

// TestSnapshotGivesUpTheLockWaitWithItsContext: a snapshot that waits
// for the store lock another holds ends with its context, here
// cancelled from the wait's seam (decided while building T060).
func TestSnapshotGivesUpTheLockWaitWithItsContext(t *testing.T) {
	f := newFixture(t)
	commit(t, f.repo, "one")
	c := f.config(t, clock)
	unlock, err := write.Lock(c.Lock, nil)
	require.NoError(t, err)
	t.Cleanup(unlock)
	ctx, cancel := context.WithCancel(t.Context())
	seam(t, &snapshotOnWait, func() { cancel() })
	_, err = Snapshot(ctx, c)
	require.ErrorIs(t, err, context.Canceled)
	_, err = os.Stat(f.snapshotDir())
	require.ErrorIs(t, err, os.ErrNotExist, "nothing written")
}

// TestSnapshotKeepsItsWorkDirectoryPrivate: the work directory is made
// 0700 on each run, whatever mode it had.
func TestSnapshotKeepsItsWorkDirectoryPrivate(t *testing.T) {
	f := newFixture(t)
	commit(t, f.repo, "one")
	c := f.config(t, clock)
	require.NoError(t, os.MkdirAll(c.Work, 0o700))
	require.NoError(t, os.Chmod(c.Work, 0o755))
	_, err := Snapshot(t.Context(), c)
	require.NoError(t, err)
	info, err := os.Stat(c.Work)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

// TestCopyAtomicCopiesARegularFileOnly: a built bundle that is a
// symbolic link or a directory is refused before it is opened.
func TestCopyAtomicCopiesARegularFileOnly(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(file, link))
	ulids := newFixture(t).config(t, clock).ULIDs
	for _, src := range []string{link, dir} {
		require.ErrorContains(t, copyAtomic(t.Context(), root, "out", src, clock, ulids), "not a regular file", src)
	}
	require.NoError(t, copyAtomic(t.Context(), root, "out", file, clock, ulids))
}

// TestAWholeSkipKeepsTheEarlierPair (decided while building T060): a
// plan that lost a whole source, here the reflog of a worktree that is
// now another repository, does not replace an earlier pair, and fails;
// with no earlier pair, it is written, and fails.
func TestAWholeSkipKeepsTheEarlierPair(t *testing.T) {
	f := newFixture(t)
	commit(t, f.repo, "one")
	_, err := Snapshot(t.Context(), f.config(t, clock))
	require.NoError(t, err)
	before, err := os.ReadFile(filepath.Join(f.snapshotDir(), salvage.HeadsFile))
	require.NoError(t, err)

	wt := filepath.Join(filepath.Dir(f.repo), "wt")
	git(t, f.repo, "worktree", "add", "-q", "-b", "w", wt)
	require.NoError(t, os.RemoveAll(wt))
	git(t, filepath.Dir(f.repo), "init", "-q", wt)
	commit(t, f.repo, "two")
	_, err = Snapshot(t.Context(), f.config(t, clock.Add(time.Minute)))
	require.ErrorIs(t, err, ErrSnapshotPartial)
	after, err := os.ReadFile(filepath.Join(f.snapshotDir(), salvage.HeadsFile))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "the earlier pair stays")

	require.NoError(t, os.RemoveAll(f.snapshotDir()))
	res, err := Snapshot(t.Context(), f.config(t, clock.Add(2*time.Minute)))
	require.ErrorIs(t, err, ErrSnapshotPartial)
	assert.True(t, res.Bundle, "with no earlier pair, the partial one is written")
	assert.Equal(t, clock.Add(2*time.Minute), f.heads(t).CreatedAt)
}

// TestSnapshotEndsAtItsDeadline: a context whose deadline has passed
// ends the snapshot with its error, and nothing is written.
func TestSnapshotEndsAtItsDeadline(t *testing.T) {
	f := newFixture(t)
	commit(t, f.repo, "one")
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()
	_, err := Snapshot(ctx, f.config(t, clock))
	require.ErrorIs(t, err, context.DeadlineExceeded)
	_, err = os.Stat(filepath.Join(f.snapshotDir(), salvage.HeadsFile))
	require.ErrorIs(t, err, os.ErrNotExist)
}

// TestCopyAtomicStopsWithItsContext: a copy whose context ends stops
// reading, fails with the context's error and leaves no temporary
// file. The context is cancelled before the copy, the one point a test
// reaches without a seam inside io.Copy.
func TestCopyAtomicStopsWithItsContext(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })
	src := filepath.Join(t.TempDir(), "src")
	require.NoError(t, os.WriteFile(src, make([]byte, 1<<20), 0o600))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err = copyAtomic(ctx, root, "out", src, clock, newFixture(t).config(t, clock).ULIDs)
	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, names(t, dir))
}
