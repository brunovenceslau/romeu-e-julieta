// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package changelog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpgrading lists the differences of the scripted fixture between
// its tag and HEAD: one id added, one exit code changed, one format
// version changed, the one kit whose embedded files changed; and the
// two one-line cases, no previous tag and nothing changed.
func TestUpgrading(t *testing.T) {
	r := scripted(t)
	cases := []struct{ name, prev, want string }{
		{"the first tag", "", "### Upgrading\n\nThere is no previous tag: this is the first release, so nothing is compared."},
		{"since v1.0.0-alpha.1", "v1.0.0-alpha.1", "### Upgrading\n\n" +
			"- Error ids added: `RJ-103` unlisted-host (exit 1).\n" +
			"- Exit codes changed: `RJ-102` drift (exit 2 to 3).\n" +
			"- Format versions changed: ProjectVersion 1 to 2.\n" +
			"- Kits whose projects need a recreate: julieta."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Upgrading(t.Context(), r.dir, tc.prev)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
	t.Run("nothing changed", func(t *testing.T) {
		r.git("tag", "v1.0.0-alpha.2")
		got, err := Upgrading(t.Context(), r.dir, "v1.0.0-alpha.2")
		require.NoError(t, err)
		assert.Equal(t, "### Upgrading\n\nNothing to do: no error id or exit code was added or changed, no format version changed and no kit changed since v1.0.0-alpha.2.", got)
	})
	t.Run("a source the previous tag lacks is empty there", func(t *testing.T) {
		// The fixture's first commit holds every source; a commit before
		// it does not exist, so the check reads the tree of the tag with
		// the page removed.
		r.git("checkout", "-q", "-b", "old", "v1.0.0-alpha.1")
		r.git("rm", "-q", errorsPage, versionsFile, kitsEmbed)
		r.commit("chore: without the sources")
		r.git("tag", "v0.9.0")
		r.git("checkout", "-q", "main")
		r.git("merge", "-q", "--no-ff", "-s", "ours", "-m", "chore: join (#9)", "old")
		got, err := Upgrading(t.Context(), r.dir, "v0.9.0")
		require.NoError(t, err)
		assert.Contains(t, got, "- Error ids added: `RJ-101` locked (exit 1), `RJ-102` drift (exit 3), `RJ-103` unlisted-host (exit 1).")
		assert.NotContains(t, got, "Format versions changed", "a version with no earlier value is not a change")
		assert.Contains(t, got, "- Kits whose projects need a recreate: git-ssh-sign, julieta, os-base.")
	})
}

// TestParseErrors reads the id, the slug and the exit code of each row
// of the error table and nothing else.
func TestParseErrors(t *testing.T) {
	page := errorsPageWith(
		"| `RJ-101` | `locked` | 1 | `wait` | [x](y) | none |",
		"| `RJ-301` | `project-approval-required` | 3 | `romeu sync <name>` | [x](y) | none: the `|` of a hint is escaped |",
		"| not a row | | | | | |")
	assert.Equal(t, errorTable{
		"RJ-101": {slug: "locked", exit: "1"},
		"RJ-301": {slug: "project-approval-required", exit: "3"},
	}, parseErrors([]byte(page)))
	assert.Empty(t, parseErrors(nil))
}

// TestParseVersions reads the integer constants named *Version of the
// source, and no other constant; an empty source is no constant.
func TestParseVersions(t *testing.T) {
	src := "package spec\n\nconst (\n\tProjectVersion = 1\n\tHostSettingsVersion = 2\n\tother = 3\n\tName = \"x\"\n)\n\nconst ManifestVersion = 4\n"
	got, err := parseVersions([]byte(src))
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"ProjectVersion": 1, "HostSettingsVersion": 2, "ManifestVersion": 4}, got)
	got, err = parseVersions(nil)
	require.NoError(t, err)
	assert.Empty(t, got)
	_, err = parseVersions([]byte("not go"))
	require.Error(t, err)
}

// TestEmbeds applies the go:embed rule to a pattern: the file itself, a
// file below a directory pattern, and no name that begins with "." or
// "_".
func TestEmbeds(t *testing.T) {
	cases := []struct {
		pattern, rel string
		want         bool
	}{
		{"julieta", "julieta/julieta.yaml", true},
		{"julieta", "julieta/bin/run.sh", true},
		{"julieta", "julieta/.hidden", false},
		{"julieta", "julieta/_skip/x", false},
		{"julieta", "julieta-claude/x.yaml", false},
		{"git-ssh-sign/git-ssh-sign.yaml", "git-ssh-sign/git-ssh-sign.yaml", true},
		{"git-ssh-sign/git-ssh-sign.yaml", "git-ssh-sign/testdata/x", false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, embeds(tc.pattern, tc.rel), "%s %s", tc.pattern, tc.rel)
	}
	patterns, err := embedPatterns([]byte("package kits\n\n//go:embed julieta os-base\n//go:embed git-ssh-sign/git-ssh-sign.yaml\nvar FS embed.FS\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{"julieta", "os-base", "git-ssh-sign/git-ssh-sign.yaml"}, patterns, "every directive is read")
	patterns, err = embedPatterns(nil)
	require.NoError(t, err)
	assert.Nil(t, patterns)
	for _, bad := range []string{"all:julieta", "kits/*.yaml", "\"a b\""} {
		_, err := embedPatterns([]byte("//go:embed " + bad + "\n"))
		assert.Error(t, err, bad)
	}
}
