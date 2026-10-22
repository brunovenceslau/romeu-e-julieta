// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	jsonv2 "encoding/json/v2"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// guardCase is a row of the table both guards are held to
// (e2e/testdata/sbx/guard-cases.jsonl), which internal/sbxdrv holds its
// own guard to as well: the differential test of the two.
type guardCase struct {
	Want string   `json:"want"`
	Argv []string `json:"argv"`
	Note string   `json:"note"`
}

// loadGuardCases reads the table.
func loadGuardCases(t *testing.T) []guardCase {
	t.Helper()
	data, err := os.ReadFile("../testdata/sbx/guard-cases.jsonl")
	require.NoError(t, err)
	var out []guardCase
	for i, line := range slices.Collect(bytes.Lines(data)) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var c guardCase
		require.NoError(t, jsonv2.Unmarshal(line, &c), "line %d", i+1)
		out = append(out, c)
	}
	require.NotEmpty(t, out)
	return out
}

// TestRefusalCases holds the rules of the fake to the shared table: it
// refuses where the table says I4 or I25, and not where it says ok. The
// rows that say shape are for the guard of sbxdrv alone.
func TestRefusalCases(t *testing.T) {
	for _, tt := range loadGuardCases(t) {
		if tt.Want == "shape" {
			continue
		}
		t.Run(tt.Want+" "+strings.Join(tt.Argv, " "), func(t *testing.T) {
			why := refusal(tt.Argv)
			if tt.Want == "ok" {
				assert.Empty(t, why)
				return
			}
			assert.True(t, strings.HasPrefix(why, tt.Want+":"), "got %q", why)
		})
	}
}
