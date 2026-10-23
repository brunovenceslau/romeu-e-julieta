// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
)

// TestPreflightSyntheticFixturesSayTheyAreSynthetic shows rule 10 of
// docs/plan.md: every file under testdata/synthetic says so in its first
// line, and the parser reads the body without it.
func TestPreflightSyntheticFixturesSayTheyAreSynthetic(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "synthetic", "*"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			raw, err := os.ReadFile(f)
			require.NoError(t, err)
			first, _, _ := bytes.Cut(raw, []byte("\n"))
			assert.True(t, strings.HasPrefix(string(first), "# synthetic: true"), "the first line is %q", first)
		})
	}
}

// TestPreflightSecretFixtures shows that each fixture that must parse
// parses with the parser of the secret listing, and that each that must
// not is upstream-shape (RJ-310), never an empty list.
func TestPreflightSecretFixtures(t *testing.T) {
	parses := map[string][]secretRow{
		"secrets-no-global.txt":           {{"sandbox:shop", "svc-alpha"}, {"sandbox:shop", "svc-beta"}},
		"secrets-global-unused.txt":       {{"global", "svc-unused"}, {"sandbox:shop", "svc-alpha"}},
		"secrets-global-used.txt":         {{"sandbox:shop", "svc-beta"}, {"global", "svc-alpha"}},
		"secrets-scope-two-sandboxes.txt": {{"sandbox:shop,sandbox:other", "svc-alpha"}},
		"secrets-scope-unknown.txt":       {{"team", "svc-alpha"}},
	}
	for name, want := range parses {
		t.Run(name, func(t *testing.T) {
			rows, err := parseSecrets(synthetic(t, name))
			require.NoError(t, err)
			assert.Equal(t, want, rows)
		})
	}
	for _, name := range []string{"secrets-changed-container.txt", "secrets-unparseable.txt", "secrets-empty.txt"} {
		t.Run(name, func(t *testing.T) {
			rows, err := parseSecrets(synthetic(t, name))
			testsupport.RequireID(t, "RJ-310", err)
			assert.Nil(t, rows)
		})
	}
	t.Run("a header alone is a listing with no secret", func(t *testing.T) {
		rows, err := parseSecrets([]byte("SCOPE  SERVICE\tSECRET\n\n"))
		require.NoError(t, err)
		assert.Empty(t, rows)
	})
	t.Run("a binary body", func(t *testing.T) {
		_, err := parseSecrets([]byte("SCOPE SERVICE SECRET\nglobal a \x00\xff\n"))
		testsupport.RequireID(t, "RJ-310", err)
	})
	t.Run("a body over the cap", func(t *testing.T) {
		_, err := parseSecrets(bytes.Repeat([]byte("x"), maxListing+1))
		testsupport.RequireID(t, "RJ-310", err)
	})
}

// TestPreflightPolicyFixtures is the same for the policy listing.
func TestPreflightPolicyFixtures(t *testing.T) {
	for _, name := range []string{
		"policies-recorded.json", "policies-global-placeholder.json", "policies-broad-doublestar.json",
		"policies-broad-tld.json", "policies-broad-default.json", "policies-broad-scope-unknown.json",
	} {
		t.Run(name, func(t *testing.T) {
			rows, err := parsePolicies(synthetic(t, name))
			require.NoError(t, err)
			assert.NotEmpty(t, rows)
		})
	}
	for _, name := range []string{
		"policies-changed-container.json", "policies-unparseable.json", "policies-unknown-kind.json", "policies-duplicate-member.json",
	} {
		t.Run(name, func(t *testing.T) {
			rows, err := parsePolicies(synthetic(t, name))
			testsupport.RequireID(t, "RJ-310", err)
			assert.Nil(t, rows)
		})
	}
	for name, body := range map[string]string{
		"an empty output":       "",
		"a missing member":      `[{"scope":"global","kind":"rule","effect":"allow"}]`,
		"a null member":         `[{"scope":null,"kind":"rule","effect":"allow","resource":"a"}]`,
		"a mistyped member":     `[{"scope":1,"kind":"rule","effect":"allow","resource":"a"}]`,
		"a null element":        `[null]`,
		"an effect not known":   `[{"scope":"global","kind":"rule","effect":"maybe","resource":"a"}]`,
		"trailing text":         `[] x`,
		"a table":               "SCOPE KIND\nglobal rule\n",
		"a renamed container":   `{"rules":[]}`,
		"a document over a cap": "[" + strings.Repeat(" ", maxListing) + "]",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parsePolicies([]byte(body))
			testsupport.RequireID(t, "RJ-310", err)
		})
	}
	t.Run("[] is an empty list", func(t *testing.T) {
		rows, err := parsePolicies([]byte(" [] \n"))
		require.NoError(t, err)
		assert.Empty(t, rows)
	})
}

// TestPreflightScopeOf shows that only exactly one sandbox is a
// per-sandbox scope (I22).
func TestPreflightScopeOf(t *testing.T) {
	tests := []struct {
		scope   string
		sandbox string
		one     bool
	}{
		{"sandbox:shop", "shop", true},
		{"global", "", false},
		{"", "", false},
		{"sandbox:", "", false},
		{"sandbox:a,sandbox:b", "", false},
		{"sandbox:*", "", false},
		{"sandbox:a b", "", false},
		{"team", "", false},
		{"Sandbox:shop", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			name, one := scopeOf(tt.scope)
			assert.Equal(t, tt.sandbox, name)
			assert.Equal(t, tt.one, one)
		})
	}
}
