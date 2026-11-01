// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHostSettingsGh holds tools.gh to REQ-1319: optional, and when
// set an absolute clean path of letters, digits and ._/+- only; an
// invalid value is the rule tool-gh, a usage error.
func TestHostSettingsGh(t *testing.T) {
	t.Parallel()
	base := func() HostSettings {
		return HostSettings{
			Schema: "host-settings/v1",
			Root:   "/srv/romeu/root",
			Config: ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"},
			Tools:  Tools{Sbx: "/usr/local/bin/sbx", Git: "/usr/bin/git"},
		}
	}
	cases := []struct {
		name string
		gh   string
		ok   bool
	}{
		{"unset", "", true},
		{"a Homebrew path", "/opt/homebrew/bin/gh", true},
		{"a path with the allowed punctuation", "/usr/local/Cellar/gh/2.102.0/bin/gh+", true},
		{"a relative path", "gh", false},
		{"a name alone", "bin/gh", false},
		{"a space", "/opt/my tools/gh", false},
		{"a shell character", "/opt/bin/gh;rm", false},
		{"a doubled slash", "/opt//bin/gh", false},
		{"a dot element", "/opt/./bin/gh", false},
		{"a parent element", "/opt/../bin/gh", false},
		{"a trailing slash", "/opt/bin/gh/", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			h := base()
			h.Tools.Gh = c.gh
			err := h.Validate()
			if c.ok {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "tools.gh")
			assert.ErrorIs(t, err, ErrInvalid)
		})
	}
}

// TestHostSettingsGhRoundTrips encodes and decodes tools.gh, and leaves
// it out of the text when unset.
func TestHostSettingsGhRoundTrips(t *testing.T) {
	t.Parallel()
	h := HostSettings{
		Schema: "host-settings/v1",
		Root:   "/srv/romeu/root",
		Config: ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"},
		Tools:  Tools{Sbx: "/usr/local/bin/sbx", Git: "/usr/bin/git"},
	}
	out, err := EncodeHostSettings(h)
	require.NoError(t, err)
	assert.NotContains(t, string(out), "gh:")
	h.Tools.Gh = "/opt/homebrew/bin/gh"
	out, err = EncodeHostSettings(h)
	require.NoError(t, err)
	assert.Contains(t, string(out), "  gh: /opt/homebrew/bin/gh\n")
	got, err := DecodeHostSettings(out)
	require.NoError(t, err)
	assert.Equal(t, "/opt/homebrew/bin/gh", got.Tools.Gh)
}
