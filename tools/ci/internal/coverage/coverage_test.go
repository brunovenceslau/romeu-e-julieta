// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package coverage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCoverageThresholds holds a fixture cover profile per threshold of
// S9: a package under 80, a package of the 90 list under 90, and both
// passing cases, each at its exact threshold.
func TestCoverageThresholds(t *testing.T) {
	tests := []struct {
		name    string
		profile []byte
		want    []string // the findings, as printed
	}{
		{
			name:    "a package under 80",
			profile: citest.ProfileOf("tools/ci 79 1", "tools/ci 21 0"),
			want:    []string{"tools/ci: coverage: 79.0% of 100 statements covered, below 80% (S9)"},
		},
		{
			name:    "a package at 80 passes",
			profile: citest.ProfileOf("tools/ci 80 1", "tools/ci 20 0"),
		},
		{
			name:    "a package of the 90 list under 90",
			profile: citest.ProfileOf("internal/gitsafe 89 3", "internal/gitsafe 11 0"),
			want:    []string{"internal/gitsafe: coverage: 89.0% of 100 statements covered, below 90% (S9)"},
		},
		{
			name:    "a package of the 90 list at 90 passes",
			profile: citest.ProfileOf("internal/gitsafe 9 1", "internal/gitsafe 1 0"),
		},
		{
			name:    "a package below a module of the 90 list is held to 90",
			profile: citest.ProfileOf("internal/state/flock 85 1", "internal/state/flock 15 0"),
			want:    []string{"internal/state/flock: coverage: 85.0% of 100 statements covered, below 90% (S9)"},
		},
		{
			name:    "a package whose name only starts like a module of the 90 list is held to 80",
			profile: citest.ProfileOf("internal/gates 85 1", "internal/gates 15 0"),
		},
		{
			name:    "a share just under the threshold does not print as the threshold",
			profile: citest.ProfileOf("tools/ci 7999 1", "tools/ci 2001 0"),
			want:    []string{"tools/ci: coverage: 79.9% of 10000 statements covered, below 80% (S9)"},
		},
		{
			name:    "a package with no test, every block at 0",
			profile: citest.ProfileOf("tools/ci/git/gittest 12 0"),
			want:    []string{"tools/ci/git/gittest: coverage: 0.0% of 12 statements covered, below 80% (S9)"},
		},
		{
			name:    "the trees of S9, each under its threshold, in the order of their paths",
			profile: citest.ProfileOf("e2e/probes 1 0", "e2e/fakesbx/x 1 0", "internal/canon 1 0", "tools/new 1 0"),
			want: []string{
				"e2e/fakesbx/x: coverage: 0.0% of 1 statements covered, below 80% (S9)",
				"e2e/probes: coverage: 0.0% of 1 statements covered, below 80% (S9)",
				"internal/canon: coverage: 0.0% of 1 statements covered, below 90% (S9)",
				"tools/new: coverage: 0.0% of 1 statements covered, below 80% (S9)",
			},
		},
		{
			name:    "a cmd package below the floor fails (S9)",
			profile: citest.ProfileOf("cmd/romeu 79 1", "cmd/romeu 21 0"),
			want:    []string{"cmd/romeu: coverage: 79.0% of 100 statements covered, below 80% (S9)"},
		},
		{
			name:    "a package outside the trees of S9 is not measured",
			profile: citest.ProfileOf("e2e/host 10 0", "e2e 10 0"),
		},
		{
			name:    "a package with no statement passes",
			profile: citest.ProfileOf("tools/types 0 0"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := coverage(tt.profile, citest.FixtureModule)
			require.NoError(t, err)
			var printed []string
			for _, f := range got {
				printed = append(printed, f.String())
			}
			assert.Equal(t, tt.want, printed)
		})
	}
}

// TestCoverageProfile reads the profile as "go test" writes it: a block
// that two test binaries list counts once, as covered when either ran
// it; a profile that is not one fails, and never reads as covered.
func TestCoverageProfile(t *testing.T) {
	twice := "mode: atomic\n" +
		citest.FixtureModule + "/tools/ci/f.go:1.2,3.4 10 0\n" +
		citest.FixtureModule + "/tools/ci/f.go:1.2,3.4 10 5\n" +
		citest.FixtureModule + "/tools/ci/f.go:5.2,6.4 90 1\n" +
		citest.FixtureModule + "/tools/ci/f.go:5.2,6.4 90 0\n"
	got, err := coverage([]byte(twice), citest.FixtureModule)
	require.NoError(t, err)
	assert.Empty(t, got, "a block listed twice counts once, covered")

	for name, bad := range map[string]string{
		"empty":           "",
		"no mode line":    citest.FixtureModule + "/tools/ci/f.go:1.2,3.4 10 0\n",
		"a short line":    "mode: set\n" + citest.FixtureModule + "/tools/ci/f.go:1.2,3.4 10\n",
		"a word count":    "mode: set\n" + citest.FixtureModule + "/tools/ci/f.go:1.2,3.4 ten 1\n",
		"a negative":      "mode: set\n" + citest.FixtureModule + "/tools/ci/f.go:1.2,3.4 10 -1\n",
		"no file name":    "mode: set\nnothing 10 1\n",
		"text, not a run": "PASS\nok\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := coverage([]byte(bad), citest.FixtureModule)
			assert.Error(t, err)
		})
	}
}

// TestCoverageModulePath reads the module line of go.mod, and of this
// repository's.
func TestCoverageModulePath(t *testing.T) {
	got, err := modulePath([]byte("// a comment\nmodule example.invalid/m\n\ngo 1.27.0\n"))
	require.NoError(t, err)
	assert.Equal(t, "example.invalid/m", got)
	got, err = modulePath([]byte("module \"example.invalid/q\"\n"))
	require.NoError(t, err)
	assert.Equal(t, "example.invalid/q", got)
	_, err = modulePath([]byte("go 1.27.0\n"))
	require.Error(t, err)

	goMod, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), "go.mod"))
	require.NoError(t, err)
	got, err = modulePath(goMod)
	require.NoError(t, err)
	assert.Equal(t, "github.com/brunovenceslau/romeu-e-julieta", got)
}

// TestCoverageCommand runs "tools/ci coverage" on a fixture profile in
// a fixture repository: exit 1 with the package below its threshold,
// exit 0 once it is covered, exit 2 without a profile.
func TestCoverageCommand(t *testing.T) {
	r := citest.NewTree(t)
	r.Write("go.mod", "module "+citest.FixtureModule+"\n\ngo 1.27.0\n")
	r.Commit("fixture")
	low := filepath.Join(t.TempDir(), "low.out")
	require.NoError(t, os.WriteFile(low, citest.ProfileOf("tools/ci 1 0"), 0o600))
	code, out := citest.Run(t, r, nil, Run, low)
	assert.Equal(t, check.ExitFail, code, out)
	assert.Contains(t, out, "tools/ci: coverage: 0.0% of 1 statements covered, below 80% (S9)\ncoverage: 1 finding(s)\n")

	r.Write("full.out", string(citest.ProfileOf("tools/ci 1 1")))
	code, out = citest.Run(t, r, nil, Run, "full.out")
	assert.Equal(t, check.ExitOK, code, out)
	assert.Equal(t, "coverage: ok\n", out)

	for _, args := range [][]string{{"coverage"}, {"coverage", "a", "b"}, {"coverage", filepath.Join(t.TempDir(), "missing.out")}} {
		code, out = citest.Run(t, r, nil, Run, args[1:]...)
		assert.Equal(t, check.ExitError, code, "%v\n%s", args, out)
	}
}
