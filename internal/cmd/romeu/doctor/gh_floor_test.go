// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/moduleroot"
)

// ghStub writes a gh stub that prints the given first line of
// gh --version and exits with status, and returns its path.
func ghStub(t *testing.T, f *fixture, line string, status int) string {
	t.Helper()
	body := "printf '%s\\nhttps://github.com/cli/cli/releases/tag/v2.102.0\\n' '" + line + "'; exit " + itoa(status)
	return stub(t, f, "gh", body)
}

func itoa(n int) string { return strconv.Itoa(n) }

// ghEnvStub writes a gh stub that prints a version at the floor only
// when its environment is exactly HOME, PATH=/usr/bin:/bin and LANG=C
// (REQ-0296), and exits 3 otherwise.
func ghEnvStub(t *testing.T, f *fixture) string {
	t.Helper()
	// The shell adds PWD, OLDPWD, SHLVL and _ of its own; the rest must
	// be the three variables.
	body := "[ \"$(env | grep -v -E '^(PWD|OLDPWD|SHLVL|_)=' | sort | tr '\\n' ' ')\" = \"HOME=" + f.home + " LANG=C PATH=/usr/bin:/bin \" ] || exit 3; printf 'gh version 2.68.0 (2025-03-11)\\n'"
	return stub(t, f, "gh", body)
}

// TestDoctorGhFloor lists the results of gh-floor (04 4.2, REQ-1331),
// one fixture per result: unset warns and names romeu upgrade and the
// install; a path that is not absolute, not there or not executable
// fails; a gh below the floor fails naming both numbers; one at or
// above passes; a gh that exits non-zero or prints another shape is an
// error.
func TestDoctorGhFloor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		arrange func(t *testing.T, f *fixture)
		want    string
		message string
	}{
		{"unset", func(_ *testing.T, f *fixture) { f.settings.Tools.Gh = "" }, resultWarn, "tools.gh is unset, so romeu upgrade cannot run: install gh at or above 2.68.0 (the install of J1 step 2)"},
		{"not there", func(_ *testing.T, f *fixture) { f.settings.Tools.Gh = filepath.Join(f.tmp, "bin", "none") }, resultFail, "does not name an executable file"},
		{"not executable", func(t *testing.T, f *fixture) {
			t.Helper()
			p := filepath.Join(f.tmp, "bin", "gh")
			require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\n"), 0o600))
			require.NoError(t, os.Chmod(p, 0o600))
			f.settings.Tools.Gh = p
		}, resultFail, "does not name an executable file"},
		{"others can write it", func(t *testing.T, f *fixture) {
			t.Helper()
			f.settings.Tools.Gh = ghStub(t, f, "gh version 2.102.0 (2026-09-30)", 0)
			require.NoError(t, os.Chmod(f.settings.Tools.Gh, 0o777))
		}, resultFail, "tools.gh is refused: "},
		{"below the floor", func(t *testing.T, f *fixture) {
			t.Helper()
			f.settings.Tools.Gh = ghStub(t, f, "gh version 2.67.0 (2025-02-19)", 0)
		}, resultFail, "gh 2.67.0 is below the floor 2.68.0"},
		{"at the floor", func(t *testing.T, f *fixture) {
			t.Helper()
			f.settings.Tools.Gh = ghStub(t, f, "gh version 2.68.0 (2025-03-11)", 0)
		}, resultPass, "gh 2.68.0, floor 2.68.0"},
		{"above the floor", func(t *testing.T, f *fixture) {
			t.Helper()
			f.settings.Tools.Gh = ghStub(t, f, "gh version 2.102.0 (2026-09-30)", 0)
		}, resultPass, "gh 2.102.0, floor 2.68.0"},
		{"a prerelease above the floor", func(t *testing.T, f *fixture) {
			t.Helper()
			f.settings.Tools.Gh = ghStub(t, f, "gh version 2.70.0-pre.1 (2025-04-01)", 0)
		}, resultPass, "gh 2.70.0-pre, floor 2.68.0"},
		{"a prerelease of the floor", func(t *testing.T, f *fixture) {
			t.Helper()
			f.settings.Tools.Gh = ghStub(t, f, "gh version 2.68.0-pre.1 (2025-03-01)", 0)
		}, resultFail, "gh 2.68.0-pre is below the floor 2.68.0"},
		{"the scrubbed environment", func(t *testing.T, f *fixture) {
			t.Helper()
			f.settings.Tools.Gh = ghEnvStub(t, f)
		}, resultPass, "gh 2.68.0, floor 2.68.0"},
		{"a long line is cut in the error", func(t *testing.T, f *fixture) {
			t.Helper()
			f.settings.Tools.Gh = ghStub(t, f, strings.Repeat("x", 5000), 0)
		}, resultError, "..."},
		{"exits non-zero", func(t *testing.T, f *fixture) {
			t.Helper()
			f.settings.Tools.Gh = ghStub(t, f, "gh version 2.102.0 (2026-09-30)", 1)
		}, resultError, "gh --version: "},
		{"another shape", func(t *testing.T, f *fixture) {
			t.Helper()
			f.settings.Tools.Gh = ghStub(t, f, "github cli 2.102.0", 0)
		}, resultError, "not a gh version line"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			c.arrange(t, f)
			rep, out, _ := f.output(t)
			got := check(t, rep, "gh-floor")
			assert.Equal(t, c.want, got.Result, out)
			assert.Contains(t, got.Message, c.message, out)
		})
	}
}

// TestGhFloorIsTheSpecs holds GhFloor to the number 12 12.1 states as
// the initial floor of gh (REQ-1058).
func TestGhFloorIsTheSpecs(t *testing.T) {
	t.Parallel()
	wd, err := os.Getwd()
	require.NoError(t, err)
	root, err := moduleroot.Find(wd)
	require.NoError(t, err)
	page, err := os.ReadFile(filepath.Join(root, "docs", "spec", "12-engineering.md"))
	require.NoError(t, err)
	m := regexp.MustCompile(`(?s)REQ-1058.{0,200}?initially ([0-9]+\.[0-9]+\.[0-9]+)`).FindSubmatch(page)
	require.NotNil(t, m, "12 12.1 states the initial floor of gh after REQ-1058")
	assert.Equal(t, GhFloor, string(m[1]))
}

// TestParseGhVersion reads the first line of gh --version and refuses
// another shape.
func TestParseGhVersion(t *testing.T) {
	t.Parallel()
	v, err := parseGhVersion([]byte("gh version 2.102.0 (2026-09-30)\nhttps://github.com/cli/cli/releases/tag/v2.102.0\n"))
	require.NoError(t, err)
	assert.Equal(t, ghVersion{nums: [3]int{2, 102, 0}}, v)
	v, err = parseGhVersion([]byte("gh version 2.70.0-pre.1 (2025-04-01)\n"))
	require.NoError(t, err)
	assert.Equal(t, ghVersion{nums: [3]int{2, 70, 0}, prerelease: true}, v)
	assert.True(t, atGhFloor(v))
	assert.False(t, atGhFloor(ghVersion{nums: [3]int{2, 68, 0}, prerelease: true}), "a prerelease of the floor is below it")
	for _, bad := range []string{"", "gh 2.1.0", "gh version 2.1", "gh version a.b.c", "gh version -1.2.3", "gh version 2.68.0-", "gh version +2.68.0"} {
		_, err := parseGhVersion([]byte(bad))
		require.Error(t, err, bad)
	}
	_, err = parseGhVersion([]byte(strings.Repeat("y", 1000)))
	require.Error(t, err)
	assert.Less(t, len(err.Error()), 300, "the line is cut before it is quoted")
}
