// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// TestSettingsPath: ROMEU_SETTINGS, then XDG_CONFIG_HOME, then HOME;
// nothing else moves it. romeu init and approve --toolchain both read
// the path through SettingsPath, so they agree on every case.
func TestSettingsPath(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want string
		err  bool
	}{
		{"override", map[string]string{"ROMEU_SETTINGS": "/a/s.yaml", "XDG_CONFIG_HOME": "/x", "HOME": "/h"}, "/a/s.yaml", false},
		{"override is cleaned", map[string]string{"ROMEU_SETTINGS": "/a//b/../s.yaml"}, "/a/s.yaml", false},
		{"override must be absolute", map[string]string{"ROMEU_SETTINGS": "s.yaml", "HOME": "/h"}, "", true},
		{"xdg", map[string]string{"XDG_CONFIG_HOME": "/x", "HOME": "/h"}, "/x/romeu/settings.yaml", false},
		{"relative xdg is unset", map[string]string{"XDG_CONFIG_HOME": "x", "HOME": "/h"}, "/h/.config/romeu/settings.yaml", false},
		{"home", map[string]string{"HOME": "/h"}, "/h/.config/romeu/settings.yaml", false},
		{"other variables", map[string]string{"ROMEU_ROOT": "/r", "ROMEU_CONFIG": "/c", "HOME": "/h"}, "/h/.config/romeu/settings.yaml", false},
		{"nothing", map[string]string{}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := spec.SettingsPath(func(k string) string { return tc.env[k] })
			if tc.err {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
