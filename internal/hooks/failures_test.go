// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks"
)

func TestFailuresRecordReplaceAndClear(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	first := hooks.Failure{At: clock, ExitCode: 1, Hook: "pre-commit", Path: "/r/.githooks/pre-commit", Repo: "/r", Stage: "tracked"}
	later := hooks.Failure{At: clock.Add(time.Hour), ExitCode: 2, Hook: "pre-commit", Path: "/r/.githooks/pre-commit", Repo: "/r", Stage: "tracked"}
	other := hooks.Failure{At: clock.Add(time.Minute), ExitCode: 3, Hook: "pre-push", Path: "x", Repo: "/r", Stage: "local"}
	for _, f := range []hooks.Failure{first, other, later} {
		require.NoError(t, hooks.Record(dir, f))
	}
	got, err := hooks.Failures(dir)
	require.NoError(t, err)
	assert.Equal(t, []hooks.Failure{other, later}, got, "one failure for each hook and repo, the oldest first")

	require.NoError(t, hooks.Clear(dir, "/r", "pre-commit"))
	require.NoError(t, hooks.Clear(dir, "/r", "pre-commit"), "clearing nothing is not an error")
	got, err = hooks.Failures(dir)
	require.NoError(t, err)
	assert.Equal(t, []hooks.Failure{other}, got)

	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestFailuresNoneRecorded(t *testing.T) {
	got, err := hooks.Failures(filepath.Join(t.TempDir(), "none"))
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestFailuresSkipsWhatItCannotTrust(t *testing.T) {
	dir := t.TempDir()
	good := hooks.Failure{At: clock, ExitCode: 1, Hook: "pre-commit", Path: "p", Repo: "/r", Stage: "tracked"}
	require.NoError(t, hooks.Record(dir, good))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pre-push-0000000000000000.json"), []byte(`{"unknown":1}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pre-commit-1111111111111111.json"), []byte(`{"at":"2026-10-20T12:00:00Z","exitCode":1,"hook":"pre-commit","path":"p","repo":"/elsewhere","stage":"tracked"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600))
	got, err := hooks.Failures(dir)
	require.Error(t, err)
	assert.Equal(t, []hooks.Failure{good}, got)
	assert.Contains(t, err.Error(), "pre-push-0000000000000000.json")
	assert.Contains(t, err.Error(), "pre-commit-1111111111111111.json")
}

func TestFailuresOfTheSessionHooks(t *testing.T) {
	dir := t.TempDir()
	start := hooks.Failure{At: clock, ExitCode: 1, Hook: hooks.SessionStart, Message: "the memory dir is gone", Repo: "", Stage: "julieta handoff show --hook"}
	end := hooks.Failure{At: clock.Add(time.Minute), ExitCode: 1, Hook: hooks.SessionEnd, Message: "git failed", Repo: "api", Stage: "julieta handoff write --facts"}
	require.NoError(t, hooks.Record(dir, start))
	require.NoError(t, hooks.Record(dir, end))
	got, err := hooks.Failures(dir)
	require.NoError(t, err)
	assert.Equal(t, []hooks.Failure{start, end}, got, "the message survives, and a failure with no repo is a failure")
	require.NoError(t, hooks.Clear(dir, "", hooks.SessionStart))
	got, err = hooks.Failures(dir)
	require.NoError(t, err)
	assert.Equal(t, []hooks.Failure{end}, got)
}
