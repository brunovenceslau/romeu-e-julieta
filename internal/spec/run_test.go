// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// TestRunLayoutIsValidated refuses a pane id used twice, two focused
// panes in a tab, a ratio outside 0.1 to 0.9, an env key on the refused
// list and a cwd with a .. element (the rows of T022), and the other
// rules of the run layout (03 3.2).
func TestRunLayoutIsValidated(t *testing.T) {
	cases := []struct {
		name, old, repl, rule string
	}{
		{"a pane id declared twice", "    - id: shell\n", "    - id: agent\n", "pane-placed-once"},
		{"a pane id placed twice", "          first: {pane: shell}\n", "          first: {pane: agent}\n", "pane-placed-once"},
		{"a pane placed nowhere", "    - id: shell\n", "    - id: spare\n      repo: shop-api\n    - id: shell\n", "pane-placed-once"},
		{"a leaf naming no pane", "          second: {pane: web}\n", "          second: {pane: nowhere}\n", "pane-placed-once"},
		{"a ratio below 0.1", "        ratio: 0.6\n", "        ratio: 0.05\n", "node-ratio"},
		{"a ratio of .nan", "        ratio: 0.6\n", "        ratio: .nan\n", "node-ratio"},
		{"a ratio of .inf", "          ratio: 0.5\n", "          ratio: .inf\n", "node-ratio"},
		{"a ratio above 0.9", "          ratio: 0.5\n", "          ratio: 0.95\n", "node-ratio"},
		{"a split neither row nor column", "        split: row\n", "        split: diagonal\n", "node-split"},
		{"a node with a pane and a split", "          first: {pane: shell}\n", "          first: {pane: shell, split: row}\n", "node-shape"},
		{"a split without its second node", "          second: {pane: web}\n", "", "node-shape"},
		{"env PATH", "      env: {PORT: \"3000\"}\n", "      env: {PATH: /x}\n", "pane-env-key"},
		{"env HOME", "      env: {PORT: \"3000\"}\n", "      env: {HOME: /x}\n", "pane-env-key"},
		{"env LD_PRELOAD", "      env: {PORT: \"3000\"}\n", "      env: {LD_PRELOAD: /x}\n", "pane-env-key"},
		{"env BASH_ENV", "      env: {PORT: \"3000\"}\n", "      env: {BASH_ENV: /x}\n", "pane-env-key"},
		{"env ENV", "      env: {PORT: \"3000\"}\n", "      env: {ENV: /x}\n", "pane-env-key"},
		{"a cwd with ..", "      cwd: app\n", "      cwd: app/../..\n", "pane-cwd"},
		{"an absolute cwd", "      cwd: app\n", "      cwd: /app\n", "pane-cwd"},
		{"a pane in no repo", "      repo: shop-web\n", "      repo: shop-docs\n", "pane-repo"},
		{"an empty step", "      steps: [[claude]]\n", "      steps: [[]]\n", "pane-step"},
		{"a bad tab label", "    - label: main\n", "    - label: Main Tab\n", "tab-label"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := spec.DecodeProject(valid(t, c.old, c.repl))
			require.ErrorIs(t, err, spec.ErrInvalid)
			assert.Contains(t, rulesOf(t, err), c.rule)
		})
	}
	t.Run("a pane placed twice while every pane is placed", func(t *testing.T) {
		data := valid(t, "  panes:\n", "    - label: more\n      root: {pane: web}\n  panes:\n")
		_, err := spec.DecodeProject(data)
		assert.Equal(t, []string{"run.panes[2].id: is placed 2 times (pane-placed-once)"}, findingLines(t, err))
	})
	t.Run("a leaf that names no declared pane", func(t *testing.T) {
		data := valid(t, "  panes:\n", "    - label: more\n      root: {pane: ghost}\n  panes:\n")
		_, err := spec.DecodeProject(data)
		assert.Equal(t, []string{"run.tabs[1].root.pane: names no declared pane (pane-placed-once)"}, findingLines(t, err))
	})
	t.Run("a tree deeper than the cap", func(t *testing.T) {
		deep := strings.Repeat("{split: row, ratio: 0.5, second: {pane: web}, first: ", 70) + "{pane: agent}" + strings.Repeat("}", 70)
		data := valid(t, "      root:\n        split: row\n", "      root: "+deep+"\n      x:\n        split: row\n")
		_, err := spec.DecodeProject(data)
		require.ErrorIs(t, err, spec.ErrInvalid)
		assert.ErrorContains(t, err, "nests deeper than")
	})
	t.Run("two focused panes in a tab", func(t *testing.T) {
		data := valid(t, "    - id: web\n", "    - id: web\n      focus: true\n")
		_, err := spec.DecodeProject(data)
		require.ErrorIs(t, err, spec.ErrInvalid)
		assert.Equal(t, []string{"tab-focus"}, rulesOf(t, err))
	})
	t.Run("one focused pane in each of two tabs", func(t *testing.T) {
		data := valid(t, "  panes:\n", "    - label: logs\n      root: {pane: logs}\n  panes:\n    - id: logs\n      repo: shop-web\n      focus: true\n")
		_, err := spec.DecodeProject(data)
		require.NoError(t, err)
	})
}
