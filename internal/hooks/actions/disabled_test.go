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

	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

// TestRecordDisabled (08 8.4, REQ-0692): with a repo-local
// core.hooksPath it makes heads.json for a repo that has none, leaves
// one that already says so alone, and sets the value of one that says
// otherwise with its refs and time kept; with none it makes no file.
func TestRecordDisabled(t *testing.T) {
	f := newFixture(t)
	later := clock.Add(time.Hour)

	disabled, err := RecordDisabled(t.Context(), f.config(t, clock))
	require.NoError(t, err)
	assert.False(t, disabled)
	assert.NoDirExists(t, f.snapshotDir(), "an enabled repo with no snapshot has nothing to say")

	git(t, f.repo, "config", "core.hooksPath", ".githooks")
	disabled, err = RecordDisabled(t.Context(), f.config(t, clock))
	require.NoError(t, err)
	assert.True(t, disabled)
	assert.Equal(t, salvage.Heads{CreatedAt: clock, Disabled: true, Refs: map[string]string{}, Schema: salvage.HeadsField}, f.heads(t))
	before, err := os.ReadFile(filepath.Join(f.snapshotDir(), salvage.HeadsFile))
	require.NoError(t, err)

	headsFile := filepath.Join(f.snapshotDir(), salvage.HeadsFile)
	infoBefore, err := os.Stat(headsFile)
	require.NoError(t, err)
	_, err = RecordDisabled(t.Context(), f.config(t, later))
	require.NoError(t, err)
	after, err := os.ReadFile(headsFile)
	require.NoError(t, err)
	assert.Equal(t, before, after, "a file that says so is left alone")
	infoAfter, err := os.Stat(headsFile)
	require.NoError(t, err)
	assert.True(t, os.SameFile(infoBefore, infoAfter), "the file was not replaced by a rename")
	assert.Equal(t, infoBefore.ModTime(), infoAfter.ModTime(), "the file was not rewritten")

	git(t, f.repo, "config", "--unset", "core.hooksPath")
	_, err = RecordDisabled(t.Context(), f.config(t, later))
	require.NoError(t, err)
	assert.Equal(t, salvage.Heads{CreatedAt: clock, Disabled: false, Refs: map[string]string{}, Schema: salvage.HeadsField}, f.heads(t))
	assert.Equal(t, []string{"heads.json"}, names(t, f.snapshotDir()), "no temporary file remains")
}

// TestRecordDisabledRefusals returns the error of a memory dir that is
// not an absolute, clean path, of a lock that cannot be taken and of a
// heads.json that does not decode, which it leaves as it is.
func TestRecordDisabledRefusals(t *testing.T) {
	f := newFixture(t)
	git(t, f.repo, "config", "core.hooksPath", ".githooks")

	c := f.config(t, clock)
	c.Memory = "relative"
	_, err := RecordDisabled(t.Context(), c)
	require.Error(t, err)

	c = f.config(t, clock)
	c.Lock = f.home // a directory
	_, err = RecordDisabled(t.Context(), c)
	require.Error(t, err)

	require.NoError(t, os.MkdirAll(f.snapshotDir(), 0o700))
	bad := filepath.Join(f.snapshotDir(), salvage.HeadsFile)
	require.NoError(t, os.WriteFile(bad, []byte("{}\n"), 0o600))
	_, err = RecordDisabled(t.Context(), f.config(t, clock))
	require.ErrorIs(t, err, salvage.ErrInvalid)
	data, err := os.ReadFile(bad)
	require.NoError(t, err)
	assert.Equal(t, "{}\n", string(data))
}

// TestRecordDisabledRefusedHooksPath counts a core.hooksPath that
// gitsafe refuses to read, an empty value or one with a control
// character, as set, as Snapshot does: the repo bypasses the dispatcher
// all the same, so heads.json says disabled (08 8.4, REQ-0692).
func TestRecordDisabledRefusedHooksPath(t *testing.T) {
	for _, tt := range []struct{ name, value string }{
		{"empty", ""},
		{"control character", "hooks\x1b[31m"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			git(t, f.repo, "config", "core.hooksPath", tt.value)
			_, set, err := f.config(t, clock).Git.HooksPath(t.Context(), f.repo)
			require.Error(t, err, "the control: gitsafe refuses the value")
			require.False(t, set)

			disabled, err := RecordDisabled(t.Context(), f.config(t, clock))
			require.NoError(t, err)
			assert.True(t, disabled)
			assert.True(t, f.heads(t).Disabled)
		})
	}
}

// TestRecordDisabledOfNoRepository is an error and writes nothing for a
// dir that is no work tree, which gitsafe refuses and which is no value
// of core.hooksPath.
func TestRecordDisabledOfNoRepository(t *testing.T) {
	f := newFixture(t)
	c := f.config(t, clock)
	c.Dir = filepath.Join(f.home, "missing")
	_, err := RecordDisabled(t.Context(), c)
	require.Error(t, err)
	assert.NoDirExists(t, f.snapshotDir())
}

// TestRecordDisabledWaitsForASnapshot parks a snapshot between its two
// writes, holding the store lock, and calls RecordDisabled with a
// short context: it returns the context's error and heads.json is as
// the snapshot left it (08 8.4, REQ-0690).
func TestRecordDisabledWaitsForASnapshot(t *testing.T) {
	f := newFixture(t)
	commit(t, f.repo, "one")
	_, err := Snapshot(t.Context(), f.config(t, clock))
	require.NoError(t, err)
	headsFile := filepath.Join(f.snapshotDir(), salvage.HeadsFile)
	before, err := os.ReadFile(headsFile)
	require.NoError(t, err)
	git(t, f.repo, "config", "core.hooksPath", ".githooks")

	parked, release := make(chan struct{}), make(chan struct{})
	seam(t, &snapshotBetweenWrites, func() error {
		close(parked)
		<-release
		return errors.New("stopped")
	})
	done := make(chan error, 1)
	snap := f.config(t, clock.Add(time.Minute))
	go func() {
		_, err := Snapshot(t.Context(), snap)
		done <- err
	}()
	<-parked
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	_, err = RecordDisabled(ctx, f.config(t, clock.Add(time.Hour)))
	require.ErrorIs(t, err, context.DeadlineExceeded)
	after, err := os.ReadFile(headsFile)
	require.NoError(t, err)
	assert.Equal(t, before, after, "heads.json is as the parked snapshot left it")

	close(release)
	require.Error(t, <-done)
	disabled, err := RecordDisabled(t.Context(), f.config(t, clock.Add(time.Hour)))
	require.NoError(t, err)
	assert.True(t, disabled)
	assert.True(t, f.heads(t).Disabled, "the lock is free again")
}
