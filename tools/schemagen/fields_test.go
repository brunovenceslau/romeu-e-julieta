// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schemagen

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFields lists the members of a format at every depth, in the order
// of the types, with the JSON path, the type in words, whether the
// member is required, and the struct field it comes from; a recursive
// type is listed once, at its first place.
func TestFields(t *testing.T) {
	got, err := Fields(reflect.TypeFor[Tree]())
	require.NoError(t, err)
	var rows [][3]string
	for _, f := range got {
		req := "optional"
		if f.Required {
			req = "required"
		}
		rows = append(rows, [3]string{f.Path, f.Type, req})
	}
	assert.Equal(t, [][3]string{
		{"schema", "string", "required"},
		{"root", "object", "required"},
		{"root.leaf", "string", "optional"},
		{"root.ratio", "number", "optional"},
		{"root.first", "object, as root", "optional"},
		{"root.next", "object, as root", "optional"},
		{"argv", "list of string", "optional"},
		{"env", "map of string to string", "optional"},
	}, rows)
	assert.Equal(t, "Leaf", got[2].Field.Name)
	assert.Equal(t, reflect.TypeFor[TreeNode](), got[2].Owner)

	fix, err := Fields(reflect.TypeFor[Fixture]())
	require.NoError(t, err)
	types := map[string]string{}
	for _, f := range fix {
		types[f.Path] = f.Type
	}
	assert.Equal(t, "list of 2 integer", types["pair"])
	assert.Equal(t, "string (date-time)", types["createdAt"])
	assert.Equal(t, "string (base64)", types["raw"])
	assert.Equal(t, "any", types["any"])
	assert.Contains(t, types, "promoted", "an embedded struct with embed adds its members in place")
}
