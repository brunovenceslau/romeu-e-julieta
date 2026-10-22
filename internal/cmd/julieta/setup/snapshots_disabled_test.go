// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

// headsFile is the snapshot/heads.json of the repo api's memory dir.
func (s *sandbox) headsFile() string {
	return filepath.Join(s.m.Repos[0].Memory, "snapshot", "heads.json")
}

// plantHeads writes a heads.json for api with the given disabled value
// and one ref, taken at at, and returns its bytes.
func (s *sandbox) plantHeads(t *testing.T, disabled bool, at time.Time) []byte {
	t.Helper()
	data, err := salvage.EncodeHeads(salvage.Heads{CreatedAt: at, Disabled: disabled,
		Refs: map[string]string{"refs/heads/main": "0123456789abcdef0123456789abcdef01234567"}})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(s.headsFile()), 0o700))
	require.NoError(t, os.WriteFile(s.headsFile(), data, 0o600))
	return data
}

// disabledWarnings returns the snapshots-disabled warnings of a report.
func disabledWarnings(rep setup.Report) []setup.Warning {
	var out []setup.Warning
	for _, w := range rep.Warnings {
		if w.Slug == setup.SnapshotsDisabled {
			out = append(out, w)
		}
	}
	return out
}

// TestSetupSnapshotsDisabled writes disabled: true into the heads.json
// of a repo whose local config sets core.hooksPath, makes the file when
// there is none, and warns snapshots-disabled for the repo's dir on
// stderr, in the report and in julieta state (08 8.4, REQ-0692).
func TestSetupSnapshotsDisabled(t *testing.T) {
	s := newSandbox(t)
	git(t, s.primary(), "config", "core.hooksPath", ".githooks")
	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, []setup.Warning{{Slug: setup.SnapshotsDisabled, Message: "snapshots disabled for api"}}, disabledWarnings(rep))
	assert.Contains(t, stderr, "warning snapshots-disabled: snapshots disabled for api\n")
	data, err := os.ReadFile(s.headsFile())
	require.NoError(t, err)
	heads, err := salvage.DecodeHeads(data)
	require.NoError(t, err)
	assert.True(t, heads.Disabled)
	assert.Equal(t, clock, heads.CreatedAt)
	assert.Empty(t, heads.Refs)
	st, err := setup.ReadState(s.home)
	require.NoError(t, err)
	assert.Equal(t, disabledWarnings(rep), disabledWarnings(setup.Report{Warnings: st.Warnings}))
	assert.NoFileExists(t, filepath.Join(s.m.Repos[1].Memory, "snapshot", "heads.json"), "web sets no hooksPath")

	code, _, stderr = julieta(t.Context(), "setup")
	require.Equal(t, 0, code, stderr)
	again, err := os.ReadFile(s.headsFile())
	require.NoError(t, err)
	assert.Equal(t, data, again, "a second setup leaves the file as it is")
}

// TestSetupSnapshotsDisabledKeepsTheSnapshot sets disabled in the
// heads.json of an earlier snapshot and keeps its refs and its time.
func TestSetupSnapshotsDisabledKeepsTheSnapshot(t *testing.T) {
	s := newSandbox(t)
	at := clock.Add(-time.Hour)
	s.plantHeads(t, false, at)
	git(t, s.primary(), "config", "core.hooksPath", ".githooks")
	code, _, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	data, err := os.ReadFile(s.headsFile())
	require.NoError(t, err)
	heads, err := salvage.DecodeHeads(data)
	require.NoError(t, err)
	assert.True(t, heads.Disabled)
	assert.Equal(t, at, heads.CreatedAt)
	assert.Equal(t, map[string]string{"refs/heads/main": "0123456789abcdef0123456789abcdef01234567"}, heads.Refs)
}

// TestSetupSnapshotsEnabled is the positive control of the two tests
// above: with no repo-local core.hooksPath setup warns nothing, makes
// no heads.json, and sets disabled back to false in one that says true.
func TestSetupSnapshotsEnabled(t *testing.T) {
	s := newSandbox(t)
	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, disabledWarnings(rep))
	assert.NotContains(t, stderr, "snapshots-disabled")
	assert.NoFileExists(t, s.headsFile())

	s.plantHeads(t, true, clock.Add(-time.Hour))
	code, rep, stderr = setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, disabledWarnings(rep))
	data, err := os.ReadFile(s.headsFile())
	require.NoError(t, err)
	heads, err := salvage.DecodeHeads(data)
	require.NoError(t, err)
	assert.False(t, heads.Disabled)
}

// TestSetupSnapshotsDisabledUnreadable lists a heads.json it cannot
// decode in the errors of the report, leaves it as it is, and still
// warns.
func TestSetupSnapshotsDisabledUnreadable(t *testing.T) {
	s := newSandbox(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(s.headsFile()), 0o700))
	require.NoError(t, os.WriteFile(s.headsFile(), []byte("{}\n"), 0o600))
	git(t, s.primary(), "config", "core.hooksPath", ".githooks")
	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	require.Len(t, rep.Errors, 1, rep.Errors)
	assert.Contains(t, rep.Errors[0], "api")
	assert.Len(t, disabledWarnings(rep), 1)
	data, err := os.ReadFile(s.headsFile())
	require.NoError(t, err)
	assert.Equal(t, "{}\n", string(data))
}
