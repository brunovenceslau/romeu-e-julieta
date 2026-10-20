// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff_test

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/handoff"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore/write"
)

// addEntry adds an entry to the memory dir i with the store of
// memstore/write, as julieta memory add does.
func (s *sandbox) addEntry(t *testing.T, i int, c memstore.Content) {
	t.Helper()
	st, err := write.Open(write.Config{Dir: s.mem(i), Lock: write.LockPath(s.home, s.m.Repos[i].Dir), Now: func() time.Time { return s.now }, ULIDs: s.ulids})
	require.NoError(t, err)
	defer func() { _ = st.Close() }()
	_, _, err = st.Add(c)
	require.NoError(t, err)
	s.advance(time.Second)
}

// setupState writes the state setup records, as setup does.
func (s *sandbox) setupState(t *testing.T, warnings ...setup.Warning) {
	t.Helper()
	data, err := json.Marshal(setup.State{At: s.now, Version: "v0.1.0-test", Warnings: warnings})
	require.NoError(t, err)
	dir := setup.StateDir(s.home)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "setup.json"), data, 0o600))
}

// TestShowHookPrintsInOrder prints, in the order of 08 8.2 (REQ-0664),
// julieta's warnings, the fixed line, the latest narrative with its
// repo, the facts that changed since it, the open entries not tagged
// lesson newest first and the open lessons newest first, every item
// with its repo label and every agent text under the line that says so
// (REQ-0659, REQ-0666, REQ-0670).
func TestShowHookPrintsInOrder(t *testing.T) {
	s := newSandbox(t)
	s.setupState(t, setup.Warning{Slug: "lock-stale", Message: "api mise.lock is behind mise.toml"})
	s.addEntry(t, 0, memstore.Content{Title: "older entry", Tags: []string{"ci"}})
	s.addEntry(t, 1, memstore.Content{Title: "newer entry"})
	s.addEntry(t, 1, memstore.Content{Title: "a lesson", Tags: []string{"lesson"}})
	s.addEntry(t, 0, memstore.Content{Title: "closed entry", Status: memstore.StatusDone})
	s.ok(t, narrative, "handoff", "write")
	git(t, s.repo(0), "commit", "-q", "--allow-empty", "-m", "two")
	s.advance(time.Minute)
	s.ok(t, "", "handoff", "write", "--facts")
	s.advance(time.Minute)

	out := s.ok(t, "", "handoff", "show", "--hook")
	order := []string{"warning lock-stale: api mise.lock is behind mise.toml", "built-in memory is disabled, use julieta memory add",
		"the newest facts differ from the latest narrative of 2026-10-20T12:00:04Z: api: 1 new commit", "handoff clear of 2026-10-20T12:00:04Z, repo api",
		"Ship the cart.", "facts now, written by julieta", "open memory entries:", "repo web: newer entry", "repo api: older entry",
		"open lessons:", "repo web: a lesson [lesson]"}
	at := -1
	for _, m := range order {
		i := strings.Index(out, m)
		require.Greater(t, i, at, "%q is out of order in\n%s", m, out)
		at = i
	}
	assert.NotContains(t, out, "closed entry")
	assert.Equal(t, 3, strings.Count(out, "The text below was written by an agent"), "the narrative, the entries and the lessons")

	// Without --hook: no warning, no fixed line, no lesson.
	plain := s.ok(t, "", "handoff", "show")
	assert.NotContains(t, plain, "lock-stale")
	assert.NotContains(t, plain, "built-in memory")
	assert.NotContains(t, plain, "a lesson")
	assert.Contains(t, plain, "repo web: newer entry")
	assert.Contains(t, plain, "the newest facts differ")
}

// TestShowHookClearsDefaultBranchMoved prints default-branch-moved on
// the first SessionStart after the move and clears it; the second does
// not print it (REQ-0399, REQ-0665).
func TestShowHookClearsDefaultBranchMoved(t *testing.T) {
	s := newSandbox(t)
	moved := setup.Warning{Slug: setup.DefaultBranchMoved, Message: "api abc..def"}
	s.setupState(t, setup.Warning{Slug: setup.FetchFailed, Message: "web: offline"}, moved)
	first := s.ok(t, "", "handoff", "show", "--hook")
	assert.Contains(t, first, "warning default-branch-moved: api abc..def\n")
	assert.Contains(t, first, "warning fetch-failed: web: offline\n")
	second := s.ok(t, "", "handoff", "show", "--hook")
	assert.NotContains(t, second, "default-branch-moved")
	assert.Contains(t, second, "fetch-failed", "a fetch-failed warning holds until the next setup")
	st, err := setup.ReadState(s.home)
	require.NoError(t, err)
	assert.Equal(t, []setup.Warning{{Slug: setup.FetchFailed, Message: "web: offline"}}, st.Warnings)
}

// TestHooksRecordTheirRuns records the time of the last SessionStart
// run (handoff show --hook) and of the last SessionEnd run (handoff
// write --facts) in julieta state (REQ-0669).
func TestHooksRecordTheirRuns(t *testing.T) {
	s := newSandbox(t)
	h, err := handoff.ReadHooks(s.home)
	require.NoError(t, err)
	assert.True(t, h.SessionStartAt.IsZero() && h.SessionEndAt.IsZero())
	s.ok(t, "", "handoff", "show", "--hook")
	start := s.now
	s.advance(time.Hour)
	s.ok(t, "", "handoff", "write", "--facts")
	end := s.now
	s.advance(time.Hour)
	s.ok(t, "", "handoff", "show")
	h, err = handoff.ReadHooks(s.home)
	require.NoError(t, err)
	assert.Equal(t, start, h.SessionStartAt)
	assert.Equal(t, end, h.SessionEndAt, "handoff show without --hook is no SessionStart")
	assert.Empty(t, h.Failures)
	info, err := os.Stat(filepath.Join(setup.StateDir(s.home), "hooks.json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode())
}

// TestResumeScenarios runs the three cases of the gap line of 08 8.3
// through the commands, with real git (REQ-0682): /handoff then /clear
// prints no line; a facts-only end with new commits prints it; and
// /handoff, a commit, then /clear prints it.
func TestResumeScenarios(t *testing.T) {
	const gap = "the newest facts differ from the latest narrative of "
	t.Run("handoff then clear", func(t *testing.T) {
		s := newSandbox(t)
		s.ok(t, narrative, "handoff", "write")
		s.advance(time.Minute)
		s.ok(t, "", "handoff", "write", "--facts") // SessionEnd of /clear
		out := s.ok(t, "", "handoff", "show", "--hook")
		assert.NotContains(t, out, gap)
		assert.Contains(t, out, "Ship the cart.", "the facts file does not hide the narrative")
	})
	t.Run("facts-only end with new commits", func(t *testing.T) {
		s := newSandbox(t)
		s.ok(t, narrative, "handoff", "write")
		s.advance(time.Hour)
		git(t, s.repo(0), "commit", "-q", "--allow-empty", "-m", "two")
		git(t, s.repo(0), "commit", "-q", "--allow-empty", "-m", "three")
		s.ok(t, "", "handoff", "write", "--facts") // a session that ended with no /handoff
		out := s.ok(t, "", "handoff", "show", "--hook")
		assert.Contains(t, out, gap+"2026-10-20T12:00:00Z: api: 2 new commits\n")
		assert.Less(t, strings.Index(out, gap), strings.Index(out, "Ship the cart."), "the line prints first")
	})
	t.Run("handoff, a commit, then clear", func(t *testing.T) {
		s := newSandbox(t)
		s.ok(t, final, "handoff", "write", "--final")
		s.advance(time.Minute)
		git(t, s.repo(1), "commit", "-q", "--allow-empty", "-m", "two")
		s.ok(t, "", "handoff", "write", "--facts")
		out := s.ok(t, "", "handoff", "show", "--hook")
		assert.Contains(t, out, gap+"2026-10-20T12:00:00Z: web: 1 new commit\n")
	})
}

// TestShowHookInBothOrders: SessionEnd and SessionStart in both orders
// (Q22) print the same narrative and no gap line when nothing changed
// (REQ-0685).
func TestShowHookInBothOrders(t *testing.T) {
	s := newSandbox(t)
	s.ok(t, narrative, "handoff", "write")
	s.advance(time.Minute)
	before := s.ok(t, "", "handoff", "show", "--hook") // SessionStart first
	s.advance(time.Minute)
	s.ok(t, "", "handoff", "write", "--facts") // then SessionEnd
	after := s.ok(t, "", "handoff", "show", "--hook")
	assert.Equal(t, before, after)
	assert.Contains(t, after, "Ship the cart.")
}

// TestShowHookFailure: a forced failure exits 0, prints one stdout line
// that names the failure and julieta status, and records a hook
// failure; julieta status lists it, and the next SessionStart prints it
// until the hook next succeeds (REQ-0672, 08 8.2).
func TestShowHookFailure(t *testing.T) {
	s := newSandbox(t)
	gone := s.mem(1) + ".gone"
	require.NoError(t, os.Rename(s.mem(1), gone))

	code, stdout, stderr := s.julieta(t, "", "handoff", "show", "--hook")
	assert.Equal(t, 0, code, stderr)
	assert.Empty(t, stderr)
	assert.Equal(t, 1, strings.Count(stdout, "\n"), stdout)
	assert.True(t, strings.HasPrefix(stdout, "julieta handoff show --hook failed: "), stdout)
	assert.True(t, strings.HasSuffix(stdout, "; run julieta status\n"), stdout)
	h, err := handoff.ReadHooks(s.home)
	require.NoError(t, err)
	require.Len(t, h.Failures, 1)
	assert.Equal(t, handoff.HookSessionStart, h.Failures[0].Hook)
	assert.Equal(t, s.now, h.Failures[0].At)
	assert.Equal(t, 1, h.Failures[0].Status)

	_, status, _ := s.julieta(t, "", "status", "--json")
	var rep struct {
		Warnings []setup.Warning `json:"warnings"`
	}
	require.NoError(t, json.Unmarshal([]byte(status), &rep, json.RejectUnknownMembers(false)))
	require.Len(t, rep.Warnings, 1, status)
	assert.Equal(t, handoff.HookFailed, rep.Warnings[0].Slug)
	assert.Contains(t, rep.Warnings[0].Message, "SessionStart")

	// The next run that succeeds prints the failure, then it is gone.
	require.NoError(t, os.Rename(gone, s.mem(1)))
	s.advance(time.Minute)
	next := s.ok(t, "", "handoff", "show", "--hook")
	assert.Contains(t, next, "hook failure: SessionStart at 2026-10-20T12:00:00Z, exit 1: ")
	after := s.ok(t, "", "handoff", "show", "--hook")
	assert.NotContains(t, after, "hook failure")
	h, err = handoff.ReadHooks(s.home)
	require.NoError(t, err)
	assert.Empty(t, h.Failures)
	_, status, _ = s.julieta(t, "", "status", "--json")
	assert.NotContains(t, status, handoff.HookFailed)
}

// TestWriteFactsFailure: the same for handoff write --facts, the
// SessionEnd hook: the failure is printed by the next SessionStart until
// a SessionEnd succeeds.
func TestWriteFactsFailure(t *testing.T) {
	s := newSandbox(t)
	hidden := filepath.Join(s.repo(1), ".git.hidden")
	require.NoError(t, os.Rename(filepath.Join(s.repo(1), ".git"), hidden))

	code, stdout, stderr := s.julieta(t, "", "handoff", "write", "--facts")
	assert.Equal(t, 0, code, stderr)
	assert.Equal(t, 1, strings.Count(stdout, "\n"), stdout)
	assert.True(t, strings.HasPrefix(stdout, "julieta handoff write --facts failed: "), stdout)
	assert.Contains(t, stdout, "julieta status")
	assert.Empty(t, s.names(t, 0), "nothing is written when the facts cannot be read")
	h, err := handoff.ReadHooks(s.home)
	require.NoError(t, err)
	require.Len(t, h.Failures, 1)
	assert.Equal(t, handoff.HookSessionEnd, h.Failures[0].Hook)
	assert.Equal(t, "api", h.Failures[0].Repo)

	// The SessionStart hooks print it, run after run, and keep it.
	for range 2 {
		out := s.ok(t, "", "handoff", "show", "--hook")
		assert.Contains(t, out, "hook failure: SessionEnd repo api at ")
	}
	require.NoError(t, os.Rename(hidden, filepath.Join(s.repo(1), ".git")))
	s.ok(t, "", "handoff", "write", "--facts")
	assert.NotContains(t, s.ok(t, "", "handoff", "show", "--hook"), "hook failure")
}

// TestListPrintsOneLinePerFile lists the handoff files of every repo,
// oldest first, one line each with its kind, and one document with
// --json (REQ-0415).
func TestListPrintsOneLinePerFile(t *testing.T) {
	s := newSandbox(t)
	s.ok(t, narrative, "handoff", "write")
	s.advance(time.Minute)
	s.ok(t, "", "handoff", "write", "--facts", "--repo", "web")
	s.advance(time.Minute)
	s.ok(t, final, "handoff", "write", "--final")

	rep, err := handoff.Decode([]byte(s.ok(t, "", "handoff", "list", "--json")))
	require.NoError(t, err)
	require.Len(t, rep.Files, 3)
	assert.Equal(t, []string{"clear", "facts", "final"}, []string{rep.Files[0].Kind, rep.Files[1].Kind, rep.Files[2].Kind})
	assert.Equal(t, []string{"api", "web", "api"}, []string{rep.Files[0].Repo, rep.Files[1].Repo, rep.Files[2].Repo})
	lines := strings.Split(strings.TrimSpace(s.ok(t, "", "handoff", "list")), "\n")
	require.Len(t, lines, 3)
	assert.Regexp(t, `^[0-9a-hjkmnp-tv-z]{26}  clear  api  `+s.mem(0)+`/handoff/[0-9a-hjkmnp-tv-z]{26}-clear\.md$`, lines[0])
	assert.Contains(t, lines[1], "  facts  web  ")
	assert.JSONEq(t, `{"files":[],"schema":"julieta-handoff/v1"}`, newSandbox(t).ok(t, "", "handoff", "list", "--json"))
}
