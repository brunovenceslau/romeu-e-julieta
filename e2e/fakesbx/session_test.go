// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const goodHeader = `{"session":"s","sbxVersion":"0.49.2","capturedAt":"2026-10-22","synthetic":true,"source":"x","description":"y"}` + "\n"

// TestLoadSession reads a session and refuses what a session may not be.
func TestLoadSession(t *testing.T) {
	tests := []struct {
		name, body, wantErr string
	}{
		{"good", goodHeader + `{"argv":["version"],"exit":0}` + "\n", ""},
		{"a blank line is skipped", goodHeader + "\n" + `{"argv":["version"],"exit":0}` + "\n", ""},
		{"a hang", goodHeader + `{"argv":["version"],"hang":true}` + "\n", ""},
		{"no header", `{"argv":["version"],"exit":0}` + "\n", "not a session header"},
		{"a header that is not JSON", "{\n", "line 1"},
		{"an entry that is not JSON", goodHeader + "{\n", "line 2"},
		{"an entry with no argv", goodHeader + `{"exit":0}` + "\n", "no argv"},
		{"an entry with neither exit nor hang", goodHeader + `{"argv":["version"]}` + "\n", "exit or hangs"},
		{"an entry with both", goodHeader + `{"argv":["version"],"exit":0,"hang":true}` + "\n", "exit or hangs"},
		{"no entry", goodHeader, "no entry"},
		{"an unknown placeholder", goodHeader + `{"argv":["stop","<nope>"],"exit":0}` + "\n", "<nope>"},
		{"an exit the fake keeps for itself", goodHeader + `{"argv":["version"],"exit":99}` + "\n", "keeps for itself"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "s.jsonl")
			require.NoError(t, os.WriteFile(path, []byte(tt.body), 0o600))
			_, err := loadSession(path)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
	_, err := loadSession(filepath.Join(t.TempDir(), "missing"))
	assert.Error(t, err)
}

// TestMutates lists the verbs that move a session on: the ones that
// change a sandbox, a policy, a secret or a setting.
func TestMutates(t *testing.T) {
	changes := [][]string{
		{"env", "run", "-d", "<dir>"}, {"env", "create", "<dir>"}, {"env", "rm", "<dir>"}, {"stop", "<name>"}, {"rm", "<name>"},
		{"policy", "allow", "network", "x"}, {"policy", "rm", "network"}, {"secret", "set", "x"}, {"settings", "set", "k", "v"},
	}
	reads := [][]string{
		{"version"}, {"ls", "--json"}, {"env", "plan", "<dir>"}, {"env", "exec", "<dir>", "--", "x"},
		{"policy", "ls"}, {"policy", "check", "network"}, {"secret", "ls"}, {"settings", "ls"}, {"env"},
	}
	for _, argv := range changes {
		assert.True(t, mutates(argv), "%q", argv)
	}
	for _, argv := range reads {
		assert.False(t, mutates(argv), "%q", argv)
	}
}
