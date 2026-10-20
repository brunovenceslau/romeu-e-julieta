// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package actions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The registry is written during initialization; a test that writes it
// restores it and does not run in parallel.
func TestForReturnsTheActionsOfAHookByName(t *testing.T) {
	saved := registry
	registry = map[string][]Action{}
	t.Cleanup(func() { registry = saved })

	noop := func(context.Context, Input) error { return nil }
	b := register("post-commit", Action{Name: "b", Run: noop})
	a := register("post-commit", Action{Name: "a", Run: noop, Continues: true})
	register("pre-commit", Action{Name: "c", Run: noop})

	got := For("post-commit")
	assert.Len(t, got, 2)
	assert.Equal(t, []string{a.Name, b.Name}, []string{got[0].Name, got[1].Name})
	assert.True(t, got[0].Continues)
	assert.Empty(t, For("pre-push"))
	assert.Len(t, For("pre-commit"), 1)
}
