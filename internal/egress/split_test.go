// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package egress_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/egress"
)

// TestSplitAccepts: a numeric port is stripped and upper case is
// lowered, in the forms the product kits write (kits/julieta,
// kits/os-base).
func TestSplitAccepts(t *testing.T) {
	tests := []struct {
		name, host, want string
	}{
		{"a port stripped", "github.com:443", "github.com"},
		{"no port", "deb.debian.org", "deb.debian.org"},
		{"upper case lowered", "Objects.GitHubUserContent.com:443", "objects.githubusercontent.com"},
		{"the highest port", "example.com:65535", "example.com"},
	}
	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.name, " ", "_"), func(t *testing.T) {
			got, err := egress.Split("julieta", []string{tt.host})
			require.NoError(t, err)
			assert.Equal(t, []string{tt.want}, got)
		})
	}
}

// TestSplitRefuses: a host that is no egress domain once its port is
// stripped, or whose port is not a number from 1 to 65535, is an error
// that names the kit and the host, and no domain comes with it.
func TestSplitRefuses(t *testing.T) {
	tests := []struct {
		name, host, want string
	}{
		{"a wildcard", "*.github.com:443", "label"},
		{"an IPv4 literal", "192.0.2.1:443", "IP literal"},
		{"an IPv6 literal with a port", "[2001:db8::1]:443", "IP literal"},
		{"a bare IPv6 literal", "[2001:db8::1]", "IP literal"},
		{"a single label", "localhost:443", "two labels"},
		{"a special-use name", "nas.local:443", "special-use"},
		{"a bad name", "git_hub.com:443", "label"},
		{"a port that is no number", "github.com:https", "port"},
		{"port zero", "github.com:0", "port"},
		{"a port above 65535", "github.com:65536", "port"},
		{"an empty port", "github.com:", "port"},
	}
	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.name, " ", "_"), func(t *testing.T) {
			got, err := egress.Split("os-base", []string{"deb.debian.org:443", tt.host})
			require.Error(t, err)
			assert.Nil(t, got)
			assert.ErrorContains(t, err, "os-base")
			assert.ErrorContains(t, err, tt.host)
			assert.ErrorContains(t, err, tt.want)
		})
	}
}
