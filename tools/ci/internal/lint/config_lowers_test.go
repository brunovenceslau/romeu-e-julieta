// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// TestLintConfigLowersNothing reads .golangci.yml and holds it to ADR
// 0007 (rules 7 and 8): testifylint with every checker, which covers
// the suite-* ones; and nothing that hides a finding. Every key of the
// file is on a short list, so a new one (an exclusion, a disabled
// linter, a plugin, a limit on the issues, a path or the tests left
// out of the run) is a decision the reader of this test sees.
func TestLintConfigLowersNothing(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), ".golangci.yml"))
	require.NoError(t, err)
	var cfg map[string]any
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	keys := func(v any) []string {
		m, ok := v.(map[string]any)
		require.True(t, ok, "a mapping, got %T", v)
		var ks []string
		for k := range m {
			ks = append(ks, k)
		}
		return ks
	}
	assert.ElementsMatch(t, []string{"version", "linters"}, keys(cfg), "top-level keys: no issues, run, output or formatters section")

	linters, ok := cfg["linters"].(map[string]any)
	require.True(t, ok, "linters")
	assert.ElementsMatch(t, []string{"enable", "settings", "exclusions"}, keys(linters),
		"linters keys: no default or disable")
	// The one exclusion key turns the skip of generated files off (ADR
	// 0016): any other key, or another value, would hide findings.
	assert.Equal(t, map[string]any{"generated": "disable"}, linters["exclusions"], "linters.exclusions: the linters read generated files")
	settings, ok := linters["settings"].(map[string]any)
	require.True(t, ok, "linters.settings")
	assert.ElementsMatch(t, []string{"revive", "gocritic", "testifylint", "unused"}, keys(settings),
		"linters.settings keys: no custom linter")
	// unused reads the objects of a generated file as it reads any
	// other (ADR 0016).
	assert.Equal(t, map[string]any{"generated-is-used": false}, settings["unused"], "unused counts no generated object as used")

	assert.Equal(t, map[string]any{"enable-all": true}, settings["testifylint"], "testifylint runs every checker")
	revive, ok := settings["revive"].(map[string]any)
	require.True(t, ok, "linters.settings.revive")
	assert.ElementsMatch(t, []string{"enable-all-rules", "rules"}, keys(revive))
	// enable-all-rules would turn on every rule of revive, each one a
	// finding of its own that ADR 0001 never asked for.
	all, ok := revive["enable-all-rules"].(bool)
	require.True(t, ok, "revive enable-all-rules is a boolean")
	assert.False(t, all, "revive enable-all-rules")
	rules, ok := revive["rules"].([]any)
	require.True(t, ok, "revive rules")
	var names []string
	for _, r := range rules {
		assert.Equal(t, []string{"name"}, keys(r), "a revive rule is its name alone: no severity, no disabled, no arguments")
		name, _ := r.(map[string]any)["name"].(string)
		names = append(names, name)
	}
	assert.Equal(t, []string{"exported", "error-strings", "package-comments"}, names, "the revive rules of ADR 0001, rule 14 and of the lint set")
	// The linters beyond the default set: the ones of ADR 0001 and ADR
	// 0007, and the lint set of item 16 of Decisions (forbidigo has its
	// own configuration, forbidigo.yml).
	assert.Equal(t, []any{"revive", "gocritic", "testifylint", "errorlint", "usetesting", "thelper"}, linters["enable"], "the enabled linters")
}
