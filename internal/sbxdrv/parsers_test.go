// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParsersRunOverTheRecordedSessions holds REQ-0930: the parsers run
// over the sessions of every recorded version, and each version has
// sessions that exercise each of them.
func TestParsersRunOverTheRecordedSessions(t *testing.T) {
	versions := map[string]int{}
	for _, s := range loadSessions(t) {
		require.Contains(t, RecordedVersions, s.Version)
		for _, e := range s.Entries {
			if e.Exit == nil || *e.Exit != 0 {
				continue
			}
			switch shapeKey(e.Argv) {
			case shapeKey(Version().Shape):
				v, err := ParseVersion([]byte(e.Stdout))
				require.NoError(t, err, s.Path)
				assert.Equal(t, s.Version, v.String(), "a session reports the version it was written for")
				require.NoError(t, CheckFloor(v))
				versions[s.Version]++
			}
		}
	}
	for _, v := range RecordedVersions {
		assert.Positive(t, versions[v], "no session of %s has sbx version", v)
	}
}

// TestPlaceholderTypes lists the placeholder types and holds each one
// to its token and to the values it accepts and refuses.
func TestPlaceholderTypes(t *testing.T) {
	tests := []struct {
		p       Placeholder
		token   string
		ok, bad string
	}{
		{sandboxName, "<name>", "shop", "Shop"},
		{envDir, "<dir>", "/abs/root/shop-env", "shop-env"},
		{domain, "<domain>", "github.com", "*.github.com"},
		{service, "<service>", "github", "GitHub"},
		{hostCommand, "<command>", "/usr/bin/security find-generic-password", "security"},
		{refresh, "<duration>", "55m", "55"},
		{settingKey, "<key>", "env.rememberHostCommands", "Env"},
		{settingValue, "<value>", "true", "a b"},
		{envAssign, "<env>", "JULIETA_MANIFEST=e30", "julieta=e30"},
		{commandWords, "<command...>", "/bin/sh", ""},
	}
	var tokens []string
	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			assert.Equal(t, tt.token, tt.p.Token)
			assert.NotEmpty(t, tt.p.What)
			require.NoError(t, tt.p.Check(tt.ok))
			require.Error(t, tt.p.Check(tt.bad))
		})
		tokens = append(tokens, tt.token)
	}
	assert.Equal(t, tokens, PlaceholderTokens(), "the list of the types is the one tested")
}
