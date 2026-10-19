// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package build

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckVersion(t *testing.T) {
	tests := []struct {
		name, version, file string
		prerelease          bool
		wantErr             string
	}{
		{"release", "v1.0.0", "1.0.0\n", false, ""},
		{"release_candidate", "v1.0.0-rc.1", "1.0.0\n", true, ""},
		{"no_v", "1.0.0", "1.0.0\n", false, "is not v<major>"},
		{"two_parts", "v1.0", "1.0.0\n", false, "is not v<major>"},
		{"leading_zero", "v01.0.0", "1.0.0\n", false, "is not v<major>"},
		{"empty_suffix", "v1.0.0-", "1.0.0\n", false, "is not v<major>"},
		{"another_version", "v1.0.1", "1.0.0\n", false, "is not a build of VERSION 1.0.0"},
		{"version_without_newline", "v1.0.0", "1.0.0", false, "VERSION holds no"},
		{"version_with_v", "v1.0.0", "v1.0.0\n", false, "VERSION holds no"},
		{"no_version_file", "v1.0.0", "", false, "VERSION holds no"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckVersion(tt.version, []byte(tt.file))
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.prerelease, got)
		})
	}
}
