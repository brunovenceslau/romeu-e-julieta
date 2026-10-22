// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package sbxdrv

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
)

// TestParseVersion reads the output of sbx version: one line with the
// version, with or without the label sbx -D prints before it.
func TestParseVersion(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{"one line", "v0.48.0\n", "0.48.0"},
		{"no v", "0.46.0", "0.46.0"},
		{"labelled", "Client Version:  v0.48.0\nServer Version:  v0.48.0\n", "0.48.0"},
		{"leading blank line", "\n\nv0.49.1\n", "0.49.1"},
		{"carriage return", "v0.48.0\r\n", "0.48.0"},
		{"prerelease", "v0.50.0-rc.1\n", "0.50.0-rc.1"},
		{"build metadata is dropped", "v0.50.0+abc123\n", "0.50.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := ParseVersion([]byte(tt.out))
			require.NoError(t, err)
			assert.Equal(t, tt.want, v.String())
		})
	}
}

// TestParseVersionShape refuses what is not a version, with
// upstream-shape (RJ-310): never a zero version.
func TestParseVersionShape(t *testing.T) {
	tests := []struct {
		name string
		out  string
	}{
		{"empty", ""},
		{"blank", " \n\n"},
		{"two components", "v0.48\n"},
		{"words", "docker sandboxes\n"},
		{"json", `{"version":"v0.48.0"}`},
		{"not a number", "v0.x.0\n"},
		{"a version and more on the line", "v0.48.0 (abc)\n"},
		{"too large", "v0.48.0" + strings.Repeat("\n", maxVersionOutput)},
		{"overflow", "v0.99999999999999999999.0\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := ParseVersion([]byte(tt.out))
			testsupport.RequireID(t, "RJ-310", err)
			assert.Equal(t, Release{}, v)
		})
	}
}

// TestUpstreamShapeDetails names sbx and the recorded versions, as
// 01 1.4 asks.
func TestUpstreamShapeDetails(t *testing.T) {
	_, err := ParseVersion([]byte("nonsense"))
	e := cli.Of(err)
	require.Equal(t, "RJ-310", e.ID)
	assert.Contains(t, e.Details, cli.Detail{Key: "upstream", Value: "sbx"})
	assert.Contains(t, e.Details, cli.Detail{Key: "recorded sbx versions", Value: strings.Join(RecordedVersions, ", ")})
}

// TestCheckFloor is the sbx floor check before gate 1.
func TestCheckFloor(t *testing.T) {
	tests := []struct {
		name  string
		out   string
		below bool
	}{
		{"the floor", "v0.46.0", false},
		{"above", "v0.48.0", false},
		{"a later major", "v1.0.0", false},
		{"one patch below", "v0.45.9", true},
		{"far below", "v0.9.0", true},
		{"a release candidate of the floor", "v0.46.0-rc.1", true},
		{"a release candidate above the floor", "v0.47.0-rc.1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := ParseVersion([]byte(tt.out))
			require.NoError(t, err)
			err = CheckFloor(v)
			if !tt.below {
				require.NoError(t, err)
				return
			}
			testsupport.RequireID(t, "RJ-343", err)
			assert.Contains(t, cli.Of(err).Details, cli.Detail{Key: "found", Value: v.String()})
			assert.Contains(t, cli.Of(err).Details, cli.Detail{Key: "floor", Value: Floor})
		})
	}
}

// TestFloorIsTheSpecs holds the floor to the one 12 12.1 states.
func TestFloorIsTheSpecs(t *testing.T) {
	src, err := os.ReadFile("../../docs/spec/12-engineering.md")
	require.NoError(t, err)
	assert.Contains(t, string(src), "`sbx` (host, >= v"+Floor+";")
}

// TestRecordedVersions holds the list to the directories under
// e2e/testdata/sbx, the tested window of 10 10.3.
func TestRecordedVersions(t *testing.T) {
	entries, err := os.ReadDir("../../e2e/testdata/sbx")
	require.NoError(t, err)
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	assert.Equal(t, dirs, RecordedVersions)
}

// TestVersionCompare orders versions by their numbers, and a
// prerelease before its release.
func TestVersionCompare(t *testing.T) {
	parse := func(s string) Release {
		v, err := ParseVersion([]byte(s))
		require.NoError(t, err)
		return v
	}
	assert.Equal(t, -1, parse("v0.46.0").Compare(parse("v0.46.1")))
	assert.Equal(t, 1, parse("v0.46.0").Compare(parse("v0.45.9")))
	assert.Equal(t, 0, parse("v0.46.0").Compare(parse("0.46.0")))
	assert.Equal(t, -1, parse("v0.46.0-rc.1").Compare(parse("v0.46.0")))
	assert.Equal(t, 1, parse("v0.46.0").Compare(parse("v0.46.0-rc.1")))
	assert.Equal(t, 0, parse("v0.46.0-rc.1").Compare(parse("v0.46.0-rc.2")), "prereleases of one release are not ordered")
}
