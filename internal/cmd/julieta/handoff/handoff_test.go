// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/handoff"
)

// TestDecodeIgnoresAnUnknownMember reads a document to which a newer
// julieta added a member, at every level, as the document without it
// (03, opening, REQ-0163).
func TestDecodeIgnoresAnUnknownMember(t *testing.T) {
	want, err := handoff.Decode([]byte(`{"files":[{"id":"01j9z3k8m6t2q4w5e7r9y1v3k5","kind":"clear","path":"/p","repo":"api"}],"schema":"julieta-handoff/v1"}`))
	require.NoError(t, err)
	got, err := handoff.Decode([]byte(`{"added":{"a":[1]},"files":[{"added":1,"id":"01j9z3k8m6t2q4w5e7r9y1v3k5","kind":"clear","path":"/p","repo":"api"}],"schema":"julieta-handoff/v1"}`))
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// TestDecodeHostileUnknownMembers holds what a member this build skips
// may not do, whoever wrote it (03, opening, REQ-0163): change what a
// known member says (a name of another case is another name, in either
// order), pass a duplicate or a nest too deep unseen, or put its own text
// in an error.
func TestDecodeHostileUnknownMembers(t *testing.T) {
	const doc = `{"files":[{"id":"01j9z3k8m6t2q4w5e7r9y1v3k5","kind":"clear","path":"/p","repo":"api"}],"schema":"julieta-handoff/v1"}`
	want, err := handoff.Decode([]byte(doc))
	require.NoError(t, err)
	after := func(members string) []byte { return []byte(strings.TrimSuffix(doc, "}") + "," + members + "}") }
	before := func(members string) []byte { return []byte("{" + members + "," + strings.TrimPrefix(doc, "{")) }
	for name, members := range map[string]string{
		"a case variant of a known name":  `"Schema":"x","SCHEMA":"y","Files":[]`,
		"an escape in a name and a value": `"\u001b[31mred":"\u0007\u001b[0m"`,
	} {
		for pos, build := range map[string]func(string) []byte{"after": after, "before": before} {
			got, err := handoff.Decode(build(members))
			require.NoError(t, err, name+" "+pos)
			assert.Equal(t, want, got, name+" "+pos)
		}
	}
	for name, members := range map[string]string{
		"a duplicate in an unknown member":        `"x":{"a":1,"a":2}`,
		"a nest of 20000 arrays":                  `"x":` + strings.Repeat("[", 20000) + strings.Repeat("]", 20000),
		"a duplicate known name beside an escape": `"\u001b[31mred":1,"schema":"julieta-handoff/v1"`,
	} {
		_, err := handoff.Decode(after(members))
		require.Error(t, err, name)
		assert.NotContains(t, err.Error(), "\x1b", name)
		assert.NotContains(t, err.Error(), "red", name)
	}
	// The name of a duplicate is in the error, as jsontext quotes it, with
	// the escape written out and no raw control character.
	_, err = handoff.Decode(after(`"\u001b[31mred":1,"\u001b[31mred":2`))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "\x1b")
}

// TestDecodeIsStrict refuses a duplicate member and another schema.
func TestDecodeIsStrict(t *testing.T) {
	_, err := handoff.Decode([]byte(`{"files":[],"schema":"julieta-handoff/v1","schema":"julieta-handoff/v1"}`))
	require.Error(t, err, "a duplicate member")
	_, err = handoff.Decode([]byte(`{"files":[],"schema":"julieta-handoff/v2"}`))
	require.Error(t, err, "another schema")
}
