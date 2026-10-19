// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package view

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tk builds a task; needs and after are ids.
func tk(id, fate string, needs []string, after ...string) Task {
	return Task{ID: id, Fate: fate, Path: id + ".md", Needs: needs, After: after}
}

func TestCriticalPath(t *testing.T) {
	for name, c := range map[string]struct {
		tasks []Task
		want  []string
	}{
		"none":         {nil, []string{}},
		"chain":        {[]Task{tk("T003", "kept", []string{"T002"}), tk("T002", "kept", []string{"T001"}), tk("T001", "kept", nil)}, []string{"T001", "T002", "T003"}},
		"after counts": {[]Task{tk("T002", "new", nil, "T001"), tk("T001", "kept", nil)}, []string{"T001", "T002"}},
		"longest wins": {[]Task{
			tk("T001", "kept", nil), tk("T002", "kept", []string{"T001"}), tk("T003", "kept", []string{"T002"}),
			tk("T004", "kept", nil), tk("T005", "kept", []string{"T004"}),
		}, []string{"T001", "T002", "T003"}},
		"tie goes to the first ids": {[]Task{
			tk("T001", "kept", nil), tk("T002", "kept", nil), tk("T003", "kept", []string{"T001"}), tk("T004", "kept", []string{"T002"}),
		}, []string{"T001", "T003"}},
		"merged adds nothing":        {[]Task{tk("T001", "merged", nil), tk("T002", "kept", []string{"T001"})}, []string{"T002"}},
		"closed edge adds nothing":   {[]Task{tk("T001", "folded", nil), tk("T002", "kept", []string{"T001"}), tk("T003", "kept", []string{"T002"})}, []string{"T002", "T003"}},
		"deferred counts":            {[]Task{tk("T001", "deferred", nil), tk("T002", "kept", []string{"T001"})}, []string{"T001", "T002"}},
		"unknown id and self edge":   {[]Task{tk("T001", "kept", []string{"T001", "T999"})}, []string{"T001"}},
		"cycle ends where it closes": {[]Task{tk("T001", "kept", []string{"T002"}), tk("T002", "kept", []string{"T001"})}, []string{"T002", "T001"}},
		"duplicate id: the first file wins": {[]Task{
			tk("T001", "kept", nil), {ID: "T001", Fate: "kept", Path: "other.md", Needs: []string{"T002"}}, tk("T002", "kept", nil),
		}, []string{"T001"}},
	} {
		t.Run(name, func(t *testing.T) { assert.Equal(t, c.want, criticalPath(c.tasks)) })
	}
}

// On the real plan the path is a chain: every id is open and once, and
// each one waits for the one before it. (tools/ci cannot be imported or run
// from a test, so the same ids are diffed by hand: see the pull request.)
func TestCriticalPathOfThePlan(t *testing.T) {
	tasks, err := load(filepath.Join("..", "..", "..", "..", TasksDir))
	require.NoError(t, err)
	got := criticalPath(tasks)
	require.NotEmpty(t, got)
	byID := first(tasks)
	seen := map[string]bool{}
	for i, id := range got {
		assert.Contains(t, open, byID[id].Fate, id)
		assert.False(t, seen[id], id)
		seen[id] = true
		if i > 0 {
			assert.Contains(t, waits(byID[id], byID), got[i-1], id)
		}
	}
}
