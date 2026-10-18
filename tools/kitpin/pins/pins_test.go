// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package pins_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/pins"
)

var (
	sumA = strings.Repeat("a", 64)
	sumB = strings.Repeat("b", 64)
)

func full() pins.Pins {
	return pins.Pins{
		Frontend: &pins.Frontend{Ref: "docker/sandbox-kit:3.0.0-m.8@sha256:" + sumA},
		Workload: &pins.Workload{
			Ref:  "docker.io/docker/sbx-kit-claude@sha256:" + sumB,
			Home: &pins.Fact{Path: "/home/agent", Source: "sitting M2"},
		},
		Herdr: &pins.Release{Version: "0.9.5", SHA256: pins.Arches{Amd64: sumA, Arm64: sumB}},
		Mise:  &pins.Release{Version: "2026.10.3", SHA256: pins.Arches{Amd64: sumB, Arm64: sumA}},
	}
}

func TestEncodeThenParse(t *testing.T) {
	src, err := pins.Encode(full())
	require.NoError(t, err)
	got, err := pins.Parse(src)
	require.NoError(t, err)
	assert.Equal(t, full(), got)
}

// TestEncodeSaysToRerunTheProbes is the comment of the acceptance that
// no check can make: it is in every file Encode writes.
func TestEncodeSaysToRerunTheProbes(t *testing.T) {
	for _, p := range []pins.Pins{{}, full()} {
		src, err := pins.Encode(p)
		require.NoError(t, err)
		assert.Contains(t, string(src), "# A pin that changes after block O3 needs the probe that read it run\n# again")
		assert.True(t, strings.HasPrefix(string(src), "# SPDX-FileCopyrightText: "))
	}
}

func TestParseEmpty(t *testing.T) {
	src, err := pins.Encode(pins.Pins{})
	require.NoError(t, err)
	got, err := pins.Parse(src)
	require.NoError(t, err)
	assert.Equal(t, pins.Pins{}, got)
}

func TestParseRefuses(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"unknown_key", "gh:\n  version: 2.0.0\n", "field gh not found"},
		{"unknown_arch", "mise:\n  version: 1.2.3\n  sha256:\n    linux-amd64: " + sumA + "\n    linux-arm64: " + sumA + "\n    darwin-arm64: " + sumA + "\n", "darwin-arm64"},
		{"two_documents", "mise: null\n---\nherdr: null\n", "more than one YAML document"},
		{"frontend_tag_only", "frontend:\n  ref: docker/sandbox-kit:3.0.0-m.8\n", "frontend.ref"},
		{"frontend_v2", "frontend:\n  ref: docker/sandbox-kit:2.0.0@sha256:" + sumA + "\n", "frontend.ref"},
		{"workload_no_registry", "workload:\n  ref: docker/sbx-kit-claude@sha256:" + sumA + "\n", "workload.ref"},
		{"home_relative", "workload:\n  ref: docker.io/a/b@sha256:" + sumA + "\n  home:\n    path: home/agent\n    source: sitting M2\n", "workload.home.path"},
		{"home_without_source", "workload:\n  ref: docker.io/a/b@sha256:" + sumA + "\n  home:\n    path: /home/agent\n    source: \"\"\n", "workload.home.source is empty"},
		{"version_latest", "herdr:\n  version: latest\n  sha256:\n    linux-amd64: " + sumA + "\n    linux-arm64: " + sumA + "\n", `herdr.version "latest"`},
		{"missing_arch", "mise:\n  version: 1.2.3\n  sha256:\n    linux-amd64: " + sumA + "\n", "mise.sha256.linux-arm64"},
		{"upper_case_hash", "mise:\n  version: 1.2.3\n  sha256:\n    linux-amd64: " + strings.ToUpper(sumA) + "\n    linux-arm64: " + sumA + "\n", "mise.sha256.linux-amd64"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := pins.Parse([]byte(c.src))
			require.ErrorContains(t, err, c.want)
		})
	}
}

func TestEncodeRefusesABadPin(t *testing.T) {
	_, err := pins.Encode(pins.Pins{Mise: &pins.Release{Version: "1.2.3"}})
	require.ErrorContains(t, err, "mise.sha256.linux-amd64")
}
