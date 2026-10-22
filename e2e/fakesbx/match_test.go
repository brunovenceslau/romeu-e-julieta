// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	jsonv2 "encoding/json/v2"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMatchShape is the unit test of the placeholder normalizer (10
// 10.1, REQ-0766): a placeholder token matches the values of its type,
// and a literal matches itself.
func TestMatchShape(t *testing.T) {
	tests := []struct {
		name  string
		shape []string
		argv  []string
		want  bool
	}{
		{"literals", []string{"ls", "--json"}, []string{"ls", "--json"}, true},
		{"another literal", []string{"ls", "--json"}, []string{"ls", "--yaml"}, false},
		{"shorter argv", []string{"ls", "--json"}, []string{"ls"}, false},
		{"longer argv", []string{"ls"}, []string{"ls", "--json"}, false},
		{"a name", []string{"stop", "<name>"}, []string{"stop", "shop"}, true},
		{"a name with a hyphen and digits", []string{"stop", "<name>"}, []string{"stop", "my-shop-2"}, true},
		{"a name in upper case", []string{"stop", "<name>"}, []string{"stop", "Shop"}, false},
		{"a name with a space", []string{"stop", "<name>"}, []string{"stop", "my shop"}, false},
		{"an empty name", []string{"stop", "<name>"}, []string{"stop", ""}, false},
		{"a name is not a flag", []string{"stop", "<name>"}, []string{"stop", "--all"}, false},
		{"a dir", []string{"env", "rm", "<dir>"}, []string{"env", "rm", "/abs/shop-env"}, true},
		{"a relative dir", []string{"env", "rm", "<dir>"}, []string{"env", "rm", "shop-env"}, false},
		{"an unclean dir", []string{"env", "rm", "<dir>"}, []string{"env", "rm", "/abs/../shop-env"}, false},
		{"a dir with a control character", []string{"env", "rm", "<dir>"}, []string{"env", "rm", "/abs/a\nb"}, false},
		{"a domain", []string{"policy", "allow", "network", "<domain>"}, []string{"policy", "allow", "network", "github.com"}, true},
		{"a wildcard domain, which romeu never writes", []string{"policy", "allow", "network", "<domain>"}, []string{"policy", "allow", "network", "*.npmjs.org:443"}, false},
		{"a domain with a space", []string{"policy", "allow", "network", "<domain>"}, []string{"policy", "allow", "network", "a b"}, false},
		{"a service", []string{"secret", "set", "<service>"}, []string{"secret", "set", "github"}, true},
		{"a service in upper case", []string{"secret", "set", "<service>"}, []string{"secret", "set", "GitHub"}, false},
		{"a command", []string{"--command", "<command>"}, []string{"--command", "/usr/bin/security find-generic-password"}, true},
		{"a relative command", []string{"--command", "<command>"}, []string{"--command", "gh auth token"}, false},
		{"a duration", []string{"--refresh", "<duration>"}, []string{"--refresh", "55m"}, true},
		{"on-demand", []string{"--refresh", "<duration>"}, []string{"--refresh", "on-demand"}, true},
		{"a bad duration", []string{"--refresh", "<duration>"}, []string{"--refresh", "55 minutes"}, false},
		{"a setting key", []string{"settings", "set", "<key>", "<value>"}, []string{"settings", "set", "env.rememberHostCommands", "true"}, true},
		{"a bad setting key", []string{"settings", "set", "<key>", "<value>"}, []string{"settings", "set", "Env", "true"}, false},
		{"a bad setting value", []string{"settings", "set", "<key>", "<value>"}, []string{"settings", "set", "a.b", "a b"}, false},
		{"an env assignment", []string{"--env", "<env>"}, []string{"--env", "JULIETA_MANIFEST=eyJ"}, true},
		{"a lower case env name", []string{"--env", "<env>"}, []string{"--env", "k=v"}, false},
		{"no equals sign", []string{"--env", "<env>"}, []string{"--env", "K"}, false},
		{"a command after the double dash", []string{"env", "exec", "<dir>", "--", "<command...>"}, []string{"env", "exec", "/d", "--", "/x/julieta", "version"}, true},
		{"one word of command", []string{"env", "exec", "<dir>", "--", "<command...>"}, []string{"env", "exec", "/d", "--", "sh"}, true},
		{"no command", []string{"env", "exec", "<dir>", "--", "<command...>"}, []string{"env", "exec", "/d", "--"}, false},
		{"no double dash", []string{"env", "exec", "<dir>", "--", "<command...>"}, []string{"env", "exec", "/d", "x", "y"}, false},
		{"a dir before the command must still match", []string{"env", "exec", "<dir>", "--", "<command...>"}, []string{"env", "exec", "d", "--", "x"}, false},
		{"a lone command token", []string{"<command...>"}, []string{"x"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, matchShape(tt.shape, tt.argv))
		})
	}
}

// TestTokens lists the placeholder tokens of a shape, in order.
func TestTokens(t *testing.T) {
	assert.Equal(t, []string{"<dir>", "<env>", "<command...>"}, tokens([]string{"env", "exec", "<dir>", "--env", "<env>", "--", "<command...>"}))
	assert.Empty(t, tokens([]string{"ls", "--json"}))
}

// placeholderCase is a row of e2e/testdata/sbx/placeholder-cases.jsonl,
// which internal/sbxdrv holds its placeholder types to as well: the
// fake matches a value of a type when the driver would build it.
type placeholderCase struct {
	Token string `json:"token"`
	Value string `json:"value"`
	Valid bool   `json:"valid"`
}

// TestPlaceholderTypesAgreeWithTheDriver runs the one table of values
// through the matchers of the fake. The fake imports nothing of
// internal/sbxdrv, so this table is what keeps the two lists of types
// equal: a value the driver refuses is not one the fake replays.
func TestPlaceholderTypesAgreeWithTheDriver(t *testing.T) {
	data, err := os.ReadFile("../testdata/sbx/placeholder-cases.jsonl")
	require.NoError(t, err)
	n := 0
	for line := range bytes.Lines(data) {
		var c placeholderCase
		require.NoError(t, jsonv2.Unmarshal(line, &c))
		n++
		matcher, ok := typeMatchers[c.Token]
		require.True(t, ok, c.Token)
		assert.Equal(t, c.Valid, matcher(c.Value), "%s %q", c.Token, c.Value)
	}
	assert.Greater(t, n, 50)
}
