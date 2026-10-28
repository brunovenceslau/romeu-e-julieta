// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schemagen

import (
	"bytes"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoRoot is the root of the repository, from this package's dir.
const repoRoot = "../.."

// admitsNull returns the JSON pointers of the places in the schema
// document v that admit null: a type null or a type list holding it, a
// const null, and an enum holding null.
func admitsNull(v any, ptr string) []string {
	var out []string
	switch v := v.(type) {
	case map[string]any:
		switch t := v["type"].(type) {
		case string:
			if t == "null" {
				out = append(out, ptr+"/type")
			}
		case []any:
			if slices.Contains(t, any("null")) {
				out = append(out, ptr+"/type")
			}
		}
		if c, ok := v["const"]; ok && c == nil {
			out = append(out, ptr+"/const")
		}
		if e, ok := v["enum"].([]any); ok && slices.Contains(e, nil) {
			out = append(out, ptr+"/enum")
		}
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			out = append(out, admitsNull(v[k], ptr+"/"+k)...)
		}
	case []any:
		for i, x := range v {
			out = append(out, admitsNull(x, fmt.Sprintf("%s/%d", ptr, i))...)
		}
	}
	return out
}

// TestNoSchemaAdmitsNull holds REQ-1273 (03 3.7) for every committed
// schema, which tools/ci schema holds equal to what the generator
// writes: no member admits null, so a member that has no value is
// omitted, never written as null. A pointer field has the schema of
// what it points to; the probe shows the walk finds a null.
func TestNoSchemaAdmitsNull(t *testing.T) {
	assert.Equal(t, []string{"#/properties/a/type", "#/properties/b/type", "#/properties/c/const", "#/properties/d/enum",
		"#/properties/e/allOf/0/prefixItems/0/type"},
		admitsNull(map[string]any{"properties": map[string]any{
			"a": map[string]any{"type": "null"}, "b": map[string]any{"type": []any{"string", "null"}},
			"c": map[string]any{"const": nil}, "d": map[string]any{"enum": []any{"x", nil}},
			"e": map[string]any{"allOf": []any{map[string]any{"prefixItems": []any{map[string]any{"type": "null"}}}}},
		}}, "#"), "the probe reaches every keyword, those keywords.go adds among them")

	files, err := filepath.Glob(filepath.Join(repoRoot, "schemas", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)
		var doc any
		require.NoError(t, jsonv2.Unmarshal(data, &doc), f)
		assert.Empty(t, admitsNull(doc, "#"), "%s admits null", filepath.Base(f))
	}

	type withPointer struct {
		Schema string        `json:"schema"`
		P      *FixtureChild `json:"p,omitzero"`
		S      *string       `json:"s"`
	}
	doc, err := Generate(Schema{Name: "pointer.v1", Type: reflect.TypeFor[withPointer]()}, "")
	require.NoError(t, err)
	var parsed any
	require.NoError(t, jsonv2.Unmarshal(doc, &parsed))
	assert.Empty(t, admitsNull(parsed, "#"), "a pointer field admits no null")
}

// TestRenderRecordValidates validates the render.json goldens of
// internal/render, which TestEncodeRecordGolden holds equal to what
// EncodeRecord writes, against schemas/render.v1.json: the record at
// rest has no promoting and a repo without lockSha256, and the record
// of a promotion commit has promoting (REQ-0226, REQ-1273). The same
// record with "promoting": null, or "lockSha256": null, is refused.
func TestRenderRecordValidates(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "schemas", "render.v1.json"))
	require.NoError(t, err)
	parsed, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	require.NoError(t, err)
	c := jsonschema.NewCompiler()
	require.NoError(t, c.AddResource("https://schemas.invalid/render.v1.json", parsed))
	sch, err := c.Compile("https://schemas.invalid/render.v1.json")
	require.NoError(t, err)
	validate := func(doc []byte) error {
		inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc))
		require.NoError(t, err)
		return sch.Validate(inst)
	}

	rest, err := os.ReadFile(filepath.Join(repoRoot, "internal", "render", "testdata", "env", "render.json"))
	require.NoError(t, err)
	promoting, err := os.ReadFile(filepath.Join(repoRoot, "internal", "render", "testdata", "env", "render-promoting.json"))
	require.NoError(t, err)
	for name, doc := range map[string][]byte{"at rest": rest, "promoting": promoting} {
		require.NoError(t, validate(doc), name)
		assert.NotContains(t, string(doc), "null", name)
	}
	require.NotContains(t, string(rest), `"promoting"`)

	withNull := strings.Replace(string(rest), `"repos":`, `"promoting":null,"repos":`, 1)
	require.NotEqual(t, string(rest), withNull)
	assert.Error(t, validate([]byte(withNull)), "promoting: null")

	var rec map[string]jsontext.Value
	require.NoError(t, jsonv2.Unmarshal(rest, &rec))
	var repos []map[string]any
	require.NoError(t, jsonv2.Unmarshal(rec["repos"], &repos))
	i := slices.IndexFunc(repos, func(r map[string]any) bool { _, ok := r["lockSha256"]; return !ok })
	require.GreaterOrEqual(t, i, 0, "a repo without lockSha256")
	repos[i]["lockSha256"] = nil
	rec["repos"], err = jsonv2.Marshal(repos)
	require.NoError(t, err)
	doc, err := jsonv2.Marshal(rec)
	require.NoError(t, err)
	assert.Error(t, validate(doc), "lockSha256: null")
}
