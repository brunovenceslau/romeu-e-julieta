// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
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

// TestSetupSnapshotsDisabledEscapesTheDir runs the step on a run whose
// repo dir holds an escape sequence, which no manifest accepts but a
// step must not trust: neither the warning nor the entry of errors
// holds a raw ESC. The CLI writer escapes what it prints and termsafe's
// Escape of an escaped text is that text, so the message escaped
// beforehand is printed once, not twice.
func TestSetupSnapshotsDisabledEscapesTheDir(t *testing.T) {
	s := newSandbox(t)
	git(t, s.primary(), "config", "core.hooksPath", ".githooks")
	hostile := "a\x1b[31mb"
	s.m.Repos = s.m.Repos[:1]
	s.m.Repos[0].Dir = hostile
	rep := setup.RecordSnapshotsDisabledForTest(t.Context(), s.home, s.m)
	require.Len(t, disabledWarnings(rep), 1)
	assert.NotContains(t, disabledWarnings(rep)[0].Message, "\x1b")
	assert.Equal(t, `snapshots disabled for a\u001b[31mb`, disabledWarnings(rep)[0].Message)
	assert.Empty(t, rep.Errors)
	require.NoError(t, os.MkdirAll(filepath.Join(s.m.Repos[0].Memory, "snapshot"), 0o700))
	require.NoError(t, os.WriteFile(s.headsFile(), []byte("{}\n"), 0o600))
	failed := setup.RecordSnapshotsDisabledForTest(t.Context(), s.home, s.m)
	require.Len(t, failed.Errors, 1)
	assert.True(t, strings.HasPrefix(failed.Errors[0], `snapshots disabled of a\u001b[31mb: `), failed.Errors[0])
	assert.NotContains(t, failed.Errors[0], "\x1b")
	var out bytes.Buffer
	setup.PrintWarning(cli.NewWriter(&out), disabledWarnings(rep)[0])
	assert.Equal(t, "warning snapshots-disabled: snapshots disabled for a\\u001b[31mb\n", out.String())
}
