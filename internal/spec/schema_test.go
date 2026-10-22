// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSchemaKeywords derives the keywords of a schema from the rules:
// each constraint JSON Schema can say, at the rule's path, and nothing
// for a rule across files or values.
func TestSchemaKeywords(t *testing.T) {
	got := map[string][]map[string]any{}
	for _, k := range SchemaKeywords(FormatProject) {
		got[k.Path] = append(got[k.Path], k.Values)
	}
	assert.Equal(t, []map[string]any{{"pattern": projectNamePattern}}, got["name"])
	assert.Equal(t, []map[string]any{{"minItems": 1}}, got["repos"])
	assert.Equal(t, []map[string]any{{"maxLength": 1024}}, got["kits[].args{}"])
	assert.Equal(t, []map[string]any{{"enum": []string{"row", "column"}}}, got["run.tabs[].root.split"])
	assert.Equal(t, []map[string]any{{"minimum": 0.1, "maximum": 0.9}}, got["run.tabs[].root.ratio"])
	assert.Equal(t, []map[string]any{{"pattern": envKeyPattern, "not": map[string]any{"pattern": `^(?:PATH|HOME|BASH_ENV|ENV|LD_.*)$`}}}, got["run.panes[].env{key}"])
	assert.Equal(t, []map[string]any{{"oneOf": []any{
		map[string]any{"required": []any{"product"}, "not": map[string]any{"anyOf": []any{map[string]any{"required": []any{"personal"}}}}},
		map[string]any{"required": []any{"personal"}, "not": map[string]any{"anyOf": []any{map[string]any{"required": []any{"product"}}}}},
	}}}, got["kits[]"])
	assert.NotContains(t, got, "projects/")
	assert.NotContains(t, got, "repos[].ref", "a Value check alone is the validators'")

	host := map[string]bool{}
	for _, k := range SchemaKeywords(FormatHostSettings) {
		host[k.Path] = true
	}
	assert.True(t, host["secrets{}.argv[0]"])
	assert.True(t, host["secrets{}.argv[]"])
}
