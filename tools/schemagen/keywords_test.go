// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schemagen

import (
	"bytes"
	jsonv2 "encoding/json/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tree is a format with a recursive struct type, as the run layout of
// project.v1 has, and the members the keywords below reach.
type Tree struct {
	Schema string            `json:"schema"`
	Root   TreeNode          `json:"root"`
	Argv   []string          `json:"argv,omitzero"`
	Env    map[string]string `json:"env,omitzero"`
}

// TreeNode is a leaf or a split of two nodes.
type TreeNode struct {
	Leaf  string    `json:"leaf,omitzero"`
	Ratio float64   `json:"ratio,omitzero"`
	First *TreeNode `json:"first,omitzero"`
	Next  *TreeNode `json:"next,omitzero"`
}

// tree is the format of Tree, with a keyword on each kind of path.
var tree = Schema{Name: "tree.v1", Type: reflect.TypeFor[Tree](), Keywords: []Keyword{
	{Path: "root", Values: map[string]any{"oneOf": []any{
		map[string]any{"required": []any{"leaf"}, "not": map[string]any{"anyOf": []any{map[string]any{"required": []any{"ratio"}}}}},
		map[string]any{"required": []any{"ratio", "first", "next"}, "not": map[string]any{"anyOf": []any{map[string]any{"required": []any{"leaf"}}}}},
	}}},
	{Path: "root.leaf", Values: map[string]any{"pattern": "^[a-z]+$"}},
	{Path: "root.ratio", Values: map[string]any{"minimum": 0.1, "maximum": 0.9}},
	{Path: "argv", Values: map[string]any{"minItems": 1}},
	{Path: "argv[0]", Values: map[string]any{"pattern": "^/"}},
	{Path: "argv[]", Values: map[string]any{"not": map[string]any{"pattern": "^ghp_"}}},
	{Path: "argv[]", Values: map[string]any{"not": map[string]any{"pattern": "^gho_"}}},
	{Path: "env{key}", Values: map[string]any{"pattern": "^[A-Z]+$"}},
	{Path: "env{}", Values: map[string]any{"maxLength": 3}},
}}

// compileTree compiles the document of tree.
func compileTree(t *testing.T) *jsonschema.Schema {
	t.Helper()
	doc, err := Generate(tree, "")
	require.NoError(t, err)
	parsed, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc))
	require.NoError(t, err)
	c := jsonschema.NewCompiler()
	c.UseLoader(jsonschema.SchemeURLLoader{})
	require.NoError(t, c.AddResource("https://schemas.invalid/tree.v1.json", parsed))
	sch, err := c.Compile("https://schemas.invalid/tree.v1.json")
	require.NoError(t, err)
	return sch
}

// TestGenerateARecursiveStruct writes a struct type reached again on its
// own path as a $ref to its first place, so a tree of any depth
// validates by one schema.
func TestGenerateARecursiveStruct(t *testing.T) {
	doc, err := Generate(tree, "")
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, jsonv2.Unmarshal(doc, &got))
	root := got["properties"].(map[string]any)["root"].(map[string]any)
	assert.Equal(t, map[string]any{"$ref": "#/properties/root"}, root["properties"].(map[string]any)["first"])

	sch := compileTree(t)
	deep := `{"schema":"tree/v1","root":{"ratio":0.5,"first":{"leaf":"a"},"next":{"ratio":0.2,"first":{"leaf":"b"},"next":{"leaf":"c"}}}}`
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(deep))
	require.NoError(t, err)
	require.NoError(t, sch.Validate(inst))
}

// TestGenerateKeywords holds each keyword to the values it reaches: an
// object, a member at any depth of a recursive type, a list, its first
// element and each element, a map's keys and values; two keywords of
// one name on one path both apply.
func TestGenerateKeywords(t *testing.T) {
	sch := compileTree(t)
	good := `{"schema":"tree/v1","root":{"leaf":"a"},"argv":["/bin/x","y"],"env":{"A":"b"}}`
	for name, doc := range map[string]string{
		"good": good,
	} {
		inst, err := jsonschema.UnmarshalJSON(strings.NewReader(doc))
		require.NoError(t, err)
		require.NoError(t, sch.Validate(inst), name)
	}
	for name, bad := range map[string]string{
		"a leaf and a ratio":                 strings.Replace(good, `{"leaf":"a"}`, `{"leaf":"a","ratio":0.5}`, 1),
		"a deep leaf":                        strings.Replace(good, `{"leaf":"a"}`, `{"ratio":0.5,"first":{"leaf":"a"},"next":{"leaf":"B"}}`, 1),
		"a deep ratio":                       strings.Replace(good, `{"leaf":"a"}`, `{"ratio":0.5,"first":{"leaf":"a"},"next":{"ratio":0.95,"first":{"leaf":"a"},"next":{"leaf":"b"}}}`, 1),
		"an empty argv":                      strings.Replace(good, `["/bin/x","y"]`, `[]`, 1),
		"a relative argv[0]":                 strings.Replace(good, `"/bin/x"`, `"x"`, 1),
		"a first element that has the shape": strings.Replace(good, `"/bin/x"`, `"ghp_x"`, 1),
		"an element of one shape":            strings.Replace(good, `"y"`, `"ghp_x"`, 1),
		"an element of the other":            strings.Replace(good, `"y"`, `"gho_x"`, 1),
		"a key":                              strings.Replace(good, `"A"`, `"a"`, 1),
		"a value":                            strings.Replace(good, `"b"`, `"bbbb"`, 1),
	} {
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			require.NotEqual(t, good, bad)
			inst, err := jsonschema.UnmarshalJSON(strings.NewReader(bad))
			require.NoError(t, err)
			assert.Error(t, sch.Validate(inst))
		})
	}
}

// TestGenerateRefusesAKeywordOffTheType fails a keyword whose path names
// no member of the type, so a renamed field cannot leave a rule behind.
func TestGenerateRefusesAKeywordOffTheType(t *testing.T) {
	for _, p := range []string{"nope", "root.nope", "argv.x", "env[]", "root{}"} {
		s := tree
		s.Keywords = []Keyword{{Path: p, Values: map[string]any{"minLength": 1}}}
		_, err := Generate(s, "")
		assert.ErrorContains(t, err, "tree.v1: the keyword path "+`"`+p+`"`, p)
	}
}
