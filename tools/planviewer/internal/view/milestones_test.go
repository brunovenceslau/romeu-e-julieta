// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package view

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const goodTable = "text\n\n| M | Name | Ends when (observable) | Delivers |\n|---|---|---|---|\n" +
	"| M0 | Shape the work | `tools/ci all` green | the shape |\n| M1 | Daily use | sitting M1 | memory |\n\nafter\n| M9 | not a row | x | y |\n"

func TestParseMilestones(t *testing.T) {
	got, err := parseMilestones(goodTable)
	require.NoError(t, err)
	assert.Equal(t, []Milestone{
		{"M0", "Shape the work", "`tools/ci all` green", "the shape"},
		{"M1", "Daily use", "sitting M1", "memory"},
	}, got)
}

func TestParseMilestonesFails(t *testing.T) {
	for name, c := range map[string]struct{ in, msg string }{
		"no table":       {"nothing\n", "no milestone table"},
		"renamed column": {"| M | Name | Done when | Delivers |\n|---|---|---|---|\n| M0 | a | b | c |\n", "want it to start with \"Ends when\""},
		"extra column":   {"| M | Name | Ends when | Delivers | Owner |\n|---|---|---|---|---|\n", "5 columns"},
		"no separator":   {"| M | Name | Ends when | Delivers |\n| M0 | a | b | c |\n", "no separator"},
		"short row":      {"| M | Name | Ends when | Delivers |\n|---|---|---|---|\n| M0 | a | b |\n", "has 3 cells"},
		"empty id":       {"| M | Name | Ends when | Delivers |\n|---|---|---|---|\n| | a | b | c |\n", "empty id"},
		"duplicate id":   {"| M | Name | Ends when | Delivers |\n|---|---|---|---|\n| M0 | a | b | c |\n| M0 | d | e | f |\n", "appears twice"},
		"no rows":        {"| M | Name | Ends when | Delivers |\n|---|---|---|---|\n", "no row"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseMilestones(c.in)
			require.ErrorContains(t, err, c.msg)
		})
	}
}

func TestMilestonesOfThePlan(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", PlanFile))
	require.NoError(t, err)
	got, err := parseMilestones(string(b))
	require.NoError(t, err)
	require.NotEmpty(t, got)
	assert.Equal(t, "M0", got[0].ID)
	for _, m := range got {
		assert.NotEmpty(t, m.Name)
		assert.NotEmpty(t, m.EndsWhen)
		assert.NotEmpty(t, m.Delivers)
	}
}

// planFile writes a plan with a one-row milestone table and returns its path.
func planFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "plan.md")
	require.NoError(t, os.WriteFile(p, []byte("| M | Name | Ends when | Delivers |\n|---|---|---|---|\n| M1 | One | done | things |\n"), 0o644))
	return p
}

func TestParseMilestonesEscapedPipe(t *testing.T) {
	got, err := parseMilestones("| M | Name | Ends when | Delivers |\n|---|---|---|---|\n| M0 | a \\| b | x \\|\\| y | z |\n")
	require.NoError(t, err)
	assert.Equal(t, []Milestone{{"M0", "a | b", "x || y", "z"}}, got)
}
