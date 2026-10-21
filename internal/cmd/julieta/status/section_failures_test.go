// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package status_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/status"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks"
)

// failuresDir is the store of hook failures below julieta state, where
// julieta hooks run and the session hooks record them.
func failuresDir(home string) string { return hooks.FailuresDir(setup.StateDir(home)) }

// TestStatusListsTheRecordedHookFailures lists each failure recorded in
// the store, a git hook's of the tracked and the system stage and a
// session hook's, with its hook, repo, stage and exit status, in --json
// and in text, the oldest first, until a later run of the same hook
// clears it (08 8.2, REQ-0675; 04 4.3, REQ-0424).
func TestStatusListsTheRecordedHookFailures(t *testing.T) {
	s := newSandbox(t)
	dir := failuresDir(s.home)
	tracked := hooks.Failure{At: clock, ExitCode: 3, Hook: "pre-commit", Path: s.primary() + "/.githooks/pre-commit", Repo: s.primary(), Stage: "tracked"}
	system := hooks.Failure{At: clock.Add(time.Minute), ExitCode: 1, Hook: "pre-push", Path: "/etc/git/hooks/pre-push", Repo: s.primary(), Stage: "system"}
	session := hooks.Failure{At: clock.Add(2 * time.Minute), ExitCode: 1, Hook: hooks.SessionStart, Message: "the memory dir\nis gone", Stage: "julieta handoff show --hook"}
	for _, f := range []hooks.Failure{session, tracked, system} {
		require.NoError(t, hooks.Record(dir, f))
	}

	rep := statusJSON(t)
	assert.Empty(t, rep.Errors)
	assert.Equal(t, []status.HookFailure{
		{At: clock, ExitCode: 3, Hook: "pre-commit", Path: tracked.Path, Repo: s.primary(), Stage: "tracked"},
		{At: system.At, ExitCode: 1, Hook: "pre-push", Path: system.Path, Repo: s.primary(), Stage: "system"},
		{At: session.At, ExitCode: 1, Hook: "SessionStart", Message: "the memory dir\nis gone", Stage: "julieta handoff show --hook"},
	}, rep.HookFailures)

	code, stdout, stderr := julieta(t.Context(), "status")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "hook failure  pre-commit  repo "+s.primary()+"  stage tracked  exit 3  at 2026-10-20T12:00:00Z  "+tracked.Path+"\n"+
		"hook failure  pre-push  repo "+s.primary()+"  stage system  exit 1  at 2026-10-20T12:01:00Z  /etc/git/hooks/pre-push\n"+
		"hook failure  SessionStart  repo (none)  stage julieta handoff show --hook  exit 1  at 2026-10-20T12:02:00Z  the memory dir\\nis gone\n",
		"one line each, a message kept on its line")
	assert.NotContains(t, stderr, "hook failure", "a recorded failure is part of the report, not a warning")

	// The dispatcher clears a hook's failure when the same hook next
	// succeeds in the same repo; status then no longer lists it.
	require.NoError(t, hooks.Clear(dir, s.primary(), "pre-commit"))
	require.NoError(t, hooks.Clear(dir, "", hooks.SessionStart))
	rep = statusJSON(t)
	require.Len(t, rep.HookFailures, 1)
	assert.Equal(t, "pre-push", rep.HookFailures[0].Hook)
}

// TestStatusWithNoHookFailures carries an empty list when the store
// holds none, or does not exist, and prints no failure line.
func TestStatusWithNoHookFailures(t *testing.T) {
	newSandbox(t)
	code, stdout, stderr := julieta(t.Context(), "status", "--json")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, `"hookFailures":[]`, "an empty list, never null")
	_, stdout, _ = julieta(t.Context(), "status")
	assert.NotContains(t, stdout, "hook failure")
}

// TestStatusPrintsAFailureWithNoDetail ends the line of a failure that
// has neither a path nor a message at its time, with no separator after
// it.
func TestStatusPrintsAFailureWithNoDetail(t *testing.T) {
	s := newSandbox(t)
	require.NoError(t, hooks.Record(failuresDir(s.home), hooks.Failure{At: clock, ExitCode: 1, Hook: hooks.SessionEnd, Repo: "api", Stage: "julieta handoff write --facts"}))
	code, stdout, stderr := julieta(t.Context(), "status")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "hook failure  SessionEnd  repo api  stage julieta handoff write --facts  exit 1  at 2026-10-20T12:00:00Z\n")
}

// TestStatusGoesOnPastAnUntrustedHookFailure lists each file of the
// store it cannot trust, one with an unknown member and one whose repo
// and hook name another file, as one entry of errors each, and the
// failures it can read as before.
func TestStatusGoesOnPastAnUntrustedHookFailure(t *testing.T) {
	s := newSandbox(t)
	dir := failuresDir(s.home)
	good := hooks.Failure{At: clock, ExitCode: 2, Hook: "pre-push", Path: "p", Repo: s.primary(), Stage: "local"}
	require.NoError(t, hooks.Record(dir, good))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pre-commit-0000000000000000.json"), []byte(`{"unknown":1}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pre-commit-1111111111111111.json"),
		[]byte(`{"at":"2026-10-20T12:00:00Z","exitCode":1,"hook":"pre-commit","path":"p","repo":"/elsewhere","stage":"tracked"}`), 0o600))

	rep := statusJSON(t)
	require.Len(t, rep.HookFailures, 1)
	assert.Equal(t, "pre-push", rep.HookFailures[0].Hook)
	require.Len(t, rep.Errors, 2, "one entry for each file: %q", rep.Errors)
	for i, name := range []string{"pre-commit-0000000000000000.json", "pre-commit-1111111111111111.json"} {
		assert.True(t, strings.HasPrefix(rep.Errors[i], "hook failures: "), rep.Errors[i])
		assert.Contains(t, rep.Errors[i], name)
		assert.NotContains(t, rep.Errors[i], "\n")
	}
}
