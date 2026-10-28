// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/oci"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
)

// TestA11ConformanceFixtures parses each conformance fixture of probe
// A11 through render, as a local kit is parsed (I28, REQ-0513; T038):
// every type sbx v0.48.0 composed at sitting M0 lands in the capability
// set the gate shows, and so does host-mount@1, which that sbx does not
// support, with its optional flag kept, so the gate shows a request sbx
// would refuse or skip rather than hide it. The fixtures are rebuilt from
// the sitting's record (source: sitting M0, reconstructed).
func TestA11ConformanceFixtures(t *testing.T) {
	t.Parallel()
	for name, check := range map[string]func(t *testing.T, c oci.Capabilities){
		"a11-network": func(t *testing.T, c oci.Capabilities) {
			t.Helper()
			require.NotNil(t, c.Network)
			assert.Equal(t, []oci.NetEntry{{Hosts: []string{"a.example"}}}, c.Network.Runtime.Allow)
		},
		"a11-volume": func(t *testing.T, c oci.Capabilities) {
			t.Helper()
			assert.Equal(t, []oci.Volume{{Kind: "volume", Path: "/data"}}, c.Volumes)
		},
		"a11-ssh-agent": func(t *testing.T, c oci.Capabilities) {
			t.Helper()
			assert.Len(t, c.SSHAgent, 1)
		},
		"a11-lifecycle": func(t *testing.T, c oci.Capabilities) {
			t.Helper()
			require.NotNil(t, c.Lifecycle)
		},
		"a11-privileged": func(t *testing.T, c oci.Capabilities) {
			t.Helper()
			assert.Equal(t, []oci.Flag{{Type: "com.docker.sandbox/privileged@1"}}, c.Flags)
		},
		"a11-host-mount": func(t *testing.T, c oci.Capabilities) {
			t.Helper()
			assert.Equal(t, []oci.Volume{{Kind: "host-mount", Path: "/m"}}, c.Volumes)
		},
		"a11-host-mount-optional": func(t *testing.T, c oci.Capabilities) {
			t.Helper()
			assert.Equal(t, []oci.Volume{{Kind: "host-mount", Path: "/m", Optional: true}}, c.Volumes)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := productSource(t, os.DirFS("testdata/kits/a11"), "/r/shop-env")
			got, err := m.Materialize(t.Context(), render.Candidate{Dir: t.TempDir()}, productSpec(name))
			require.NoError(t, err)
			require.Len(t, got, 1)
			assert.Equal(t, "mixin", got[0].Capabilities.Kind)
			check(t, got[0].Capabilities)
		})
	}
	entries, err := os.ReadDir("testdata/kits/a11")
	require.NoError(t, err)
	assert.Len(t, entries, 8, "seven fixtures, one per capability type, and REUSE.toml")
}
