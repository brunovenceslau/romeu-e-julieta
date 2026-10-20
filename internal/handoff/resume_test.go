// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/handoff"
)

// commitsAB is the count of a test: three commits between head a and
// head b of the repo api, unknown anywhere else.
func commitsAB(repo, from, to string) (int, bool) {
	if repo == "api" && from == head('a') && to == head('b') {
		return 3, true
	}
	return 0, false
}

// resume returns the SessionStart print over the memory dir mem, as the
// hook prints it.
func resume(t *testing.T, mem string) string {
	t.Helper()
	l, err := handoff.ReadLatest([]handoff.Dir{{Label: "api", Path: mem}})
	require.NoError(t, err)
	return handoff.Session{Hook: true, Latest: l, Count: commitsAB, Limit: handoff.DefaultLimit}.Render()
}

// TestResumeHandoffThenClear: /handoff writes a narrative, and the
// SessionEnd hook of /clear writes a facts file with the same git
// facts; the next SessionStart prints the narrative and no gap line
// (REQ-0682, the first golden). The facts file is newer than the
// narrative and does not hide it (REQ-0685).
func TestResumeHandoffThenClear(t *testing.T) {
	mem := t.TempDir()
	put(t, mem, 1*time.Minute, file(handoff.Clear, 1*time.Minute, api('a', 2)))
	put(t, mem, 2*time.Minute, file(handoff.Facts, 2*time.Minute, api('a', 2)))
	got := resume(t, mem)
	assert.NotContains(t, got, "the newest facts differ")
	golden(t, "resume-handoff-then-clear.golden", got)
}

// TestResumeFactsOnlyEnd: the last session ended without /handoff and
// with new commits, so its facts file differs from the narrative of
// the session before it: the gap line prints first (REQ-0682, the
// second golden).
func TestResumeFactsOnlyEnd(t *testing.T) {
	mem := t.TempDir()
	put(t, mem, 1*time.Minute, file(handoff.Clear, 1*time.Minute, api('a', 2)))
	put(t, mem, 5*time.Hour, file(handoff.Facts, 5*time.Hour, api('b', 0)))
	got := resume(t, mem)
	assert.Contains(t, got, "the newest facts differ from the latest narrative of 2026-10-02T18:31:00Z: api: 3 new commits; api: dirty 2 -> 0\n")
	golden(t, "resume-facts-only-end.golden", got)
}

// TestResumeHandoffCommitThenClear: /handoff, a commit, then /clear:
// the facts file at /clear differs from the narrative, so the line
// prints though the narrative is the freshest handoff (REQ-0682, the
// third golden).
func TestResumeHandoffCommitThenClear(t *testing.T) {
	mem := t.TempDir()
	put(t, mem, 1*time.Minute, file(handoff.Final, 1*time.Minute, api('a', 0)))
	put(t, mem, 2*time.Minute, file(handoff.Facts, 2*time.Minute, api('b', 0)))
	got := resume(t, mem)
	assert.Contains(t, got, "the newest facts differ from the latest narrative of 2026-10-02T18:31:00Z: api: 3 new commits\n")
	golden(t, "resume-handoff-commit-then-clear.golden", got)
}

// TestResumeInBothOrders: SessionStart of the new session may run
// before or after the SessionEnd of the old one (Q22). Before it, the
// facts file is not there and the print holds the narrative; after it,
// the print holds the same narrative and, with the same git facts, no
// line (REQ-0685).
func TestResumeInBothOrders(t *testing.T) {
	mem := t.TempDir()
	put(t, mem, 1*time.Minute, file(handoff.Clear, 1*time.Minute, api('a', 2)))
	before := resume(t, mem)
	put(t, mem, 2*time.Minute, file(handoff.Facts, 2*time.Minute, api('a', 2)))
	after := resume(t, mem)
	assert.Equal(t, before, after)
	assert.Contains(t, after, "handoff clear of 2026-10-02T18:31:00Z, repo api")
}

// TestResumeIgnoresOlderFacts: a facts file older than the narrative
// says nothing about what happened since it.
func TestResumeIgnoresOlderFacts(t *testing.T) {
	mem := t.TempDir()
	put(t, mem, 1*time.Minute, file(handoff.Facts, 1*time.Minute, api('b', 9)))
	put(t, mem, 2*time.Minute, file(handoff.Clear, 2*time.Minute, api('a', 2)))
	assert.NotContains(t, resume(t, mem), "the newest facts differ")
}

// TestChanges words each kind of difference, and a head whose commits
// cannot be counted as moved.
func TestChanges(t *testing.T) {
	old := []handoff.RepoFacts{api('a', 1), {Dir: "gone", Head: head('c')}}
	moved := api('b', 4)
	moved.Branch = "main"
	moved.Stashes = 1
	moved.Ahead, moved.Behind = 0, 1
	got := handoff.Changes(old, []handoff.RepoFacts{moved, {Dir: "new"}}, commitsAB)
	assert.Equal(t, []string{
		"api: 3 new commits",
		"api: branch feat/cart -> main",
		"api: dirty 1 -> 4",
		"gone: removed from the manifest",
		"new: added to the manifest",
	}, got)
	pushed := api('a', 1)
	pushed.Ahead, pushed.Stashes = 0, 3
	assert.Empty(t, handoff.Changes([]handoff.RepoFacts{api('a', 1)}, []handoff.RepoFacts{pushed}, nil), "a push or a stash is no gap")
	assert.Equal(t, []string{"api: the head moved"}, handoff.Changes([]handoff.RepoFacts{api('a', 0)}, []handoff.RepoFacts{api('c', 0)}, commitsAB))
	assert.Empty(t, handoff.Changes(old, old, nil))
}
