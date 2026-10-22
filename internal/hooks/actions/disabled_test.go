// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package actions

import (
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

	_, err = RecordDisabled(t.Context(), f.config(t, later))
	require.NoError(t, err)
	after, err := os.ReadFile(filepath.Join(f.snapshotDir(), salvage.HeadsFile))
	require.NoError(t, err)
	assert.Equal(t, before, after, "a file that says so is left alone")

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
	c.Lock = filepath.Join(f.home, "no", "such", "dir", "lock")
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
