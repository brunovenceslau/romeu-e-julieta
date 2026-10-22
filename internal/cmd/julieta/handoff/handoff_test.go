// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff_test

import (
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

// TestDecodeIsStrict refuses a duplicate member and another schema.
func TestDecodeIsStrict(t *testing.T) {
	_, err := handoff.Decode([]byte(`{"files":[],"schema":"julieta-handoff/v1","schema":"julieta-handoff/v1"}`))
	require.Error(t, err, "a duplicate member")
	_, err = handoff.Decode([]byte(`{"files":[],"schema":"julieta-handoff/v2"}`))
	require.Error(t, err, "another schema")
}
