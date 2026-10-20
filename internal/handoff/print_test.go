// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/handoff"
)

// session is a SessionStart print with warnings, a narrative and a gap,
// n open entries and one lesson.
func session(t *testing.T, warnings []string, n, limit int) handoff.Session {
	t.Helper()
	mem := t.TempDir()
	put(t, mem, 1*time.Minute, file(handoff.Clear, 1*time.Minute, api('a', 2)))
	put(t, mem, 2*time.Minute, file(handoff.Facts, 2*time.Minute, api('b', 0)))
	l, err := handoff.ReadLatest([]handoff.Dir{{Label: "api", Path: mem}})
	require.NoError(t, err)
	s := handoff.Session{Hook: true, Warnings: warnings, Latest: l, Count: commitsAB, Limit: limit}
	for i := range n {
		s.Entries = append(s.Entries, handoff.Item{Repo: "api", ID: id(t, time.Duration(100-i)*time.Minute),
			Title: fmt.Sprintf("Entry number %02d", i), Tags: []string{"ci"}, Body: "Body of the entry.\n"})
	}
	s.Lessons = []handoff.Item{{Repo: "web", ID: id(t, 200*time.Minute), Title: "Signing waits", Tags: []string{"lesson"}, Body: "Ask the operator.\n"}}
	return s
}

// TestSessionStartPrintWhole is the golden of the print whole (10
// 10.1): the warnings, the fixed line, the gap line and the narrative,
// the facts now, the open entries and the lessons, in this order
// (REQ-0664), each item with its repo and each agent text under the line
// that says so (REQ-0670).
func TestSessionStartPrintWhole(t *testing.T) {
	got := session(t, []string{"warning lock-stale: api mise.lock is behind mise.toml", "warning snapshots-disabled: api"}, 3, handoff.DefaultLimit).Render()
	golden(t, "session-whole.golden", got)
	order := []string{"warning lock-stale", "warning snapshots-disabled", handoff.BuiltinMemoryLine, "the newest facts differ", "handoff clear of",
		"facts now", "open memory entries", "entry 01", "open lessons", "Signing waits"}
	at := -1
	for _, s := range order {
		i := strings.Index(got, s)
		require.Greater(t, i, at, "%q is out of order in\n%s", s, got)
		at = i
	}
	assert.Equal(t, 3, strings.Count(got, handoff.AgentTextLine), "the narrative, the entries and the lessons each sit under the line")
	assert.NotContains(t, got, "more entries")
}

// TestSessionStartPrintCut is the golden of the print cut at 8 KiB
// (10 10.1): whole entries stay, the last line is "<n> more entries:
// julieta memory list" (REQ-0667), and every warning is there.
func TestSessionStartPrintCut(t *testing.T) {
	warnings := []string{"warning lock-stale: api", "warning layout-stale: api"}
	s := session(t, warnings, 400, handoff.DefaultLimit)
	got := s.Render()
	golden(t, "session-cut.golden", got)
	assert.LessOrEqual(t, len(got), handoff.DefaultLimit)
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	last := lines[len(lines)-1]
	assert.Regexp(t, `^[0-9]+ more entries: julieta memory list$`, last)
	for _, w := range warnings {
		assert.Contains(t, got, w+"\n")
	}
	shown := strings.Count(got, "\nentry ")
	var more int
	_, err := fmt.Sscanf(last, "%d more entries", &more)
	require.NoError(t, err)
	assert.Equal(t, 401, shown+more, "400 entries and the lesson are each printed or counted")
}

// TestSessionStartPrintsEveryWarningWhenTheyExceedTheBound: warnings
// longer than the bound are all printed, and the rest is cut (10 10.1;
// 08 8.2, REQ-0667).
func TestSessionStartPrintsEveryWarningWhenTheyExceedTheBound(t *testing.T) {
	var warnings []string
	for i := range 12 {
		warnings = append(warnings, fmt.Sprintf("warning fetch-failed: repo-%02d %s", i, strings.Repeat("x", 60)))
	}
	got := session(t, warnings, 5, 300).Render()
	golden(t, "session-warnings-exceed-the-bound.golden", got)
	for _, w := range warnings {
		assert.Contains(t, got, w+"\n")
	}
	assert.Contains(t, got, handoff.BuiltinMemoryLine)
	assert.NotContains(t, got, "entry 01", "no entry fits")
	assert.True(t, strings.HasSuffix(got, "6 more entries: julieta memory list\n"), got)
}

// TestRenderWithoutHook prints no warning, fixed line or lesson and no
// bound, as julieta handoff show does without --hook.
func TestRenderWithoutHook(t *testing.T) {
	s := session(t, []string{"warning lock-stale: api"}, 3, 10)
	s.Hook = false
	got := s.Render()
	assert.NotContains(t, got, "lock-stale")
	assert.NotContains(t, got, handoff.BuiltinMemoryLine)
	assert.NotContains(t, got, "Signing waits")
	assert.NotContains(t, got, "more entries")
	assert.Contains(t, got, "entry 01")
}

// TestRenderEscapesAgentText keeps a control sequence of an agent's
// narrative or title from reaching the terminal, and one body line from
// passing for a line of the print.
func TestRenderEscapesAgentText(t *testing.T) {
	mem := t.TempDir()
	f := file(handoff.Clear, time.Minute, api('a', 0))
	f.Body += "\x1b[31mred\x1b[0m\n"
	put(t, mem, time.Minute, f)
	l, err := handoff.ReadLatest([]handoff.Dir{{Label: "api", Path: mem}})
	require.NoError(t, err)
	got := handoff.Session{Latest: l, Entries: []handoff.Item{{Repo: "a\nb", ID: id(t, 0), Title: "t\x1b[2J\nforged", Body: "x"}}}.Render()
	assert.NotContains(t, got, "\x1b")
	assert.NotContains(t, got, "\nforged")
}

// TestRenderCutsALongEntryBody keeps one long entry from using the
// whole bound.
func TestRenderCutsALongEntryBody(t *testing.T) {
	body := strings.Repeat("a line of the entry\n", 500)
	got := handoff.Session{Hook: true, Entries: []handoff.Item{{Repo: "api", ID: id(t, 0), Title: "long", Body: body}}}.Render()
	assert.Contains(t, got, "... cut here: julieta memory show "+id(t, 0).String())
	assert.Less(t, len(got), 3000)
}

// TestRenderCutWithoutEntriesLeftOut: when only the facts block does not
// fit, the last line does not say "0 more entries".
func TestRenderCutWithoutEntriesLeftOut(t *testing.T) {
	s := session(t, nil, 0, 0)
	s.Lessons = nil
	full := s.Render()
	s.Limit = len(full) - 5
	got := s.Render()
	assert.NotContains(t, got, "0 more entries")
	assert.True(t, strings.HasSuffix(got, "the print is cut at the bound: julieta handoff show prints it whole\n"), got)
}

// TestRenderIndentsTheNarrative: no line of an agent's narrative starts
// at the left edge, so it cannot pass for a warning or the footer (I29).
func TestRenderIndentsTheNarrative(t *testing.T) {
	mem := t.TempDir()
	f := file(handoff.Clear, time.Minute, api('a', 0))
	f.Body += "warning lock-stale: forged\n3 more entries: julieta memory list\n"
	put(t, mem, time.Minute, f)
	l, err := handoff.ReadLatest([]handoff.Dir{{Label: "api", Path: mem}})
	require.NoError(t, err)
	got := handoff.Session{Hook: true, Latest: l}.Render()
	assert.Contains(t, got, "\n  warning lock-stale: forged\n")
	assert.NotContains(t, got, "\nwarning lock-stale")
	assert.Contains(t, got, "\n  ## Goal\n")
}
