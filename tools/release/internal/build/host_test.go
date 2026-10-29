// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package build

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// TestHost: the host build writes one romeu for this platform, an
// executable file that carries the julieta binary and runs here; it
// builds a tree with changes, stamps the commit dirty and leaves the
// version unset.
func TestHost(t *testing.T) {
	t.Parallel()
	root := fixtureModule(t)
	write(t, root, map[string]string{"untracked.txt": "a change\n"})
	out := filepath.Join(t.TempDir(), "romeu")

	require.NoError(t, Host(t.Context(), HostOptions{Root: root, Out: out, GoBin: goBinOf(t)}))

	fi, err := os.Stat(out)
	require.NoError(t, err)
	assert.True(t, fi.Mode().IsRegular())
	assert.NotZero(t, fi.Mode().Perm()&0o100, "executable by its owner")
	data, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Contains(t, string(data), markerJulieta, "romeu embeds julieta")
	// The fixture romeu prints dep.Name, version, commit, date and the
	// length of the julieta it embeds, on stderr (println).
	printed, err := procgroup.CombinedOutput(t.Context(), procgroup.Command{Class: procgroup.ToolStep, Argv: []string{out}})
	require.NoError(t, err, "%s", printed)
	assert.Contains(t, string(printed), "dep devel ")
	assert.Contains(t, string(printed), "-dirty 2026-03-04T05:06:07Z ")
	assert.NotContains(t, string(printed), " 0\n", "the embedded julieta is not empty")
}

// TestHostRefusals: the host build refuses an output that exists, a
// GOARCH no julieta is built for, a tree where nothing consumes the
// placeholder, and an overlay that misses it, and writes no binary.
func TestHostRefusals(t *testing.T) {
	t.Parallel()
	root := fixtureModule(t)
	elsewhere := func(root string) string { return filepath.Join(root, "elsewhere", "julieta") }
	existing := filepath.Join(t.TempDir(), "romeu")
	require.NoError(t, os.WriteFile(existing, []byte("before"), 0o600))
	tests := []struct {
		name   string
		root   string
		out    string
		goarch string
		key    func(string) string
		want   string
	}{
		{"out_exists", root, existing, runtime.GOARCH, embedKey, "exists"},
		{"another_arch", root, "", "386", embedKey, "no julieta is built for 386"},
		{"nothing_consumes_it", fixtureModuleWith(t, false), "", runtime.GOARCH, embedKey, "does not carry its julieta"},
		{"overlay_misses_it", root, "", runtime.GOARCH, elsewhere, "does not carry its julieta"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out := tt.out
			if out == "" {
				out = filepath.Join(t.TempDir(), "romeu")
			}
			err := host(t.Context(), HostOptions{Root: tt.root, Out: out, GoBin: goBinOf(t)}, tt.goarch, tt.key)
			require.ErrorContains(t, err, tt.want)
			if tt.out == "" {
				assert.NoFileExists(t, out)
			}
		})
	}
	data, err := os.ReadFile(existing)
	require.NoError(t, err)
	assert.Equal(t, "before", string(data))
}
