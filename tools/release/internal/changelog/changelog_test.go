// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package changelog

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite the golden section of testdata/golden")

// golden is the section the scripted fixture renders for v1.0.0-alpha.2,
// where the review of the template reads its output whole: one entry
// per line, the Why under it, every issue and pull request a link, the
// breaking entries first, the Upgrading part from the fixture's
// differences.
const golden = "testdata/golden/v1.0.0-alpha.2.md"

// TestRunRendersTheSection renders the section of the scripted fixture
// with --stdout and holds it to the golden; the run writes nothing.
func TestRunRendersTheSection(t *testing.T) {
	r := scripted(t)
	before := r.read(File)
	var out bytes.Buffer
	require.NoError(t, Run(t.Context(), Options{Root: r.dir, Version: "v1.0.0-alpha.2", Cliff: cliffPath(t), Stdout: true, Out: &out}))
	assert.Equal(t, before, r.read(File), "a --stdout run writes nothing")
	if *update {
		require.NoError(t, os.MkdirAll(filepath.Dir(golden), 0o755))
		require.NoError(t, os.WriteFile(golden, out.Bytes(), 0o644))
	}
	want, err := os.ReadFile(golden)
	require.NoError(t, err, "run with -update to write the golden")
	assert.Equal(t, string(want), out.String())
	assert.NotContains(t, out.String(), "chore(release)", "the release pull request is skipped")
	assert.NotContains(t, out.String(), "merge: main into", "an in-branch merge of the merge type is skipped")
	assert.NotContains(t, out.String(), "Merge remote-tracking", "an in-branch merge under git's subject is skipped")
	assert.NotContains(t, out.String(), "outside a pull request", "a commit that is no merge is skipped")
	assert.NotContains(t, out.String(), "the project spec reader", "an entry before the previous tag is not in the section")
	assert.NotContains(t, out.String(), "The alternative.", "a heading that begins with Why is not the Why section")
	assert.NotContains(t, out.String(), "A second Why", "the first Why section is the one read")
	assert.NotContains(t, out.String(), "<!--", "an HTML comment of the body is not read")
	assert.Contains(t, out.String(), "\n  The reason.\n")
}

// TestRunWritesTheFile prepends the section to CHANGELOG.md: the header
// once, one blank line after its marker, the new section before the
// older one with one blank line between, one newline at the end; and
// a second run for the same tag refuses, so two runs give one file.
func TestRunWritesTheFile(t *testing.T) {
	r := scripted(t)
	cliff := cliffPath(t)
	header := r.read(File)
	require.True(t, strings.HasSuffix(header, Marker+"\n"), "the committed file ends with the marker")

	require.NoError(t, Run(t.Context(), Options{Root: r.dir, Version: "v1.0.0-alpha.2", Cliff: cliff}))
	file := r.read(File)
	assert.True(t, strings.HasPrefix(file, header+"\n## v1.0.0-alpha.2\n"), "the header, one blank line, the section:\n%s", file)
	assert.Equal(t, 1, strings.Count(file, Marker), "the marker once")
	assert.True(t, strings.HasSuffix(file, "\n") && !strings.HasSuffix(file, "\n\n"), "one newline at the end: %q", file[len(file)-8:])
	section, ok := Section([]byte(file), "v1.0.0-alpha.2")
	require.True(t, ok)
	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	assert.Equal(t, strings.TrimRight(string(want), "\n"), string(section), "the section in the file is the rendered one")

	err = Run(t.Context(), Options{Root: r.dir, Version: "v1.0.0-alpha.2", Cliff: cliff})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already holds the section of v1.0.0-alpha.2")
	assert.Equal(t, file, r.read(File), "the refusal writes nothing")

	// A later tag goes before the earlier one, with one blank line.
	r.git("tag", "v1.0.0-alpha.2")
	r.pr(8, "feat(upgrade): romeu upgrade", "## Why\n\nOne command installs a release.\n", map[string]string{"h.txt": "h\n"})
	require.NoError(t, Run(t.Context(), Options{Root: r.dir, Version: "v1.0.0-alpha.3", Cliff: cliff}))
	file = r.read(File)
	assert.True(t, strings.HasPrefix(file, header+"\n## v1.0.0-alpha.3\n"), file)
	i := strings.Index(file, "\n## v1.0.0-alpha.2\n")
	require.Positive(t, i)
	assert.True(t, strings.HasSuffix(file[:i], ".\n") && !strings.HasSuffix(file[:i], "\n\n"), "one blank line before the older section: %q", file[i-20:i+20])
	assert.Len(t, headingsOf(file), 2, "two sections")
	third, ok := Section([]byte(file), "v1.0.0-alpha.3")
	require.True(t, ok)
	assert.Contains(t, string(third), "### Upgrading\n\nNothing to do:")
	assert.Contains(t, string(third), "- upgrade: romeu upgrade ([#8](https://github.com/brunovenceslau/romeu-e-julieta/issues/8))")
}

// headingsOf returns the level-2 headings of file.
func headingsOf(file string) []string {
	var out []string
	for _, m := range heading.FindAllStringSubmatch(file, -1) {
		out = append(out, m[1])
	}
	return out
}

// TestRunRefusals lists what the command refuses, each by its message,
// with the file unchanged.
func TestRunRefusals(t *testing.T) {
	r := scripted(t)
	cliff := cliffPath(t)
	r.git("push", "-q", "origin", "v1.0.0-alpha.1")
	// A tag that is on origin alone.
	r.git("tag", "v1.0.0-rc.1")
	r.git("push", "-q", "origin", "v1.0.0-rc.1")
	r.git("tag", "-d", "v1.0.0-rc.1")
	before := r.read(File)
	cases := []struct {
		name, version, want string
	}{
		{"not a tag", "1.0.0", "is not v<major>.<minor>.<patch>"},
		{"not of VERSION", "v2.0.0-alpha.1", "is not a build of VERSION 1.0.0"},
		{"a suffix that is not alpha or rc", "v1.0.0-beta.1", "suffix is not alpha.<n> or rc.<n>"},
		{"a suffix counting from zero", "v1.0.0-alpha.0", "suffix is not alpha.<n> or rc.<n>"},
		{"a tag of the repository", "v1.0.0-alpha.1", "exists on this repository"},
		{"a tag of origin", "v1.0.0-rc.1", "exists on origin"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Run(t.Context(), Options{Root: r.dir, Version: tc.version, Cliff: cliff})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.Equal(t, before, r.read(File))
		})
	}
	t.Run("no entry since the previous tag", func(t *testing.T) {
		r.git("tag", "v1.0.0-alpha.2")
		err := Run(t.Context(), Options{Root: r.dir, Version: "v1.0.0-alpha.3", Cliff: cliff})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no merged pull request since v1.0.0-alpha.2")
		assert.Equal(t, before, r.read(File))
	})
	t.Run("a change outside a pull request alone", func(t *testing.T) {
		// The Upgrading part lists the change, which is no entry.
		r.write(map[string]string{versionsFile: versionsWith(3, 1)})
		r.commit("chore: a version bump outside a pull request")
		err := Run(t.Context(), Options{Root: r.dir, Version: "v1.0.0-alpha.3", Cliff: cliff})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no merged pull request since v1.0.0-alpha.2")
		assert.Equal(t, before, r.read(File))
	})
	t.Run("git-cliff fails", func(t *testing.T) {
		r.write(map[string]string{Config: "[changelog]\nbody = \"{{ not closed\"\n"})
		err := Run(t.Context(), Options{Root: r.dir, Version: "v1.0.0-alpha.3", Cliff: cliff})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--offline --no-exec", "the argv is in the error")
		assert.Contains(t, err.Error(), "TemplateParseError", "git-cliff's standard error is in the error, escaped")
		assert.Contains(t, err.Error(), "\\x1b[", "the escapes of git-cliff's output are written as text")
		assert.Equal(t, before, r.read(File))
		r.git("checkout", "-q", "--", Config)
	})
	t.Run("no marker", func(t *testing.T) {
		r.write(map[string]string{File: "# Changelog\n"})
		err := Run(t.Context(), Options{Root: r.dir, Version: "v1.0.0-alpha.3", Cliff: cliff})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "holds no header marker")
	})
	t.Run("no origin", func(t *testing.T) {
		r.git("remote", "remove", "origin")
		err := Run(t.Context(), Options{Root: r.dir, Version: "v1.0.0-alpha.3", Cliff: cliff})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no remote origin")
	})
}

// TestRunFirstTag renders the section of a repository with no tag: the
// Upgrading part says there is no previous tag, and the entries are
// every pull request.
func TestRunFirstTag(t *testing.T) {
	r := newRepo(t)
	r.pr(1, "feat(init): romeu init", "## Why\n\nA host is set up once.\n", map[string]string{"a.txt": "a\n"})
	var out bytes.Buffer
	require.NoError(t, Run(t.Context(), Options{Root: r.dir, Version: "v1.0.0-alpha.1", Cliff: cliffPath(t), Stdout: true, Out: &out}))
	assert.Contains(t, out.String(), "### Upgrading\n\nThere is no previous tag: this is the first release, so nothing is compared.\n")
	assert.Contains(t, out.String(), "- init: romeu init ([#1](")
	assert.NotContains(t, out.String(), "the previous stable tag", "a prerelease has no link to the earlier sections")
}

// TestRunStableTagLinksTheFile renders a tag without a suffix: its
// section links CHANGELOG.md at the tag, where the prereleases are.
func TestRunStableTagLinksTheFile(t *testing.T) {
	r := newRepo(t)
	r.pr(1, "feat(init): romeu init", "## Why\n\nA host is set up once.\n", map[string]string{"a.txt": "a\n"})
	var out bytes.Buffer
	require.NoError(t, Run(t.Context(), Options{Root: r.dir, Version: "v1.0.0", Cliff: cliffPath(t), Stdout: true, Out: &out}))
	assert.Contains(t, out.String(), "## v1.0.0\n\n[The documentation at this tag](https://github.com/brunovenceslau/romeu-e-julieta/tree/v1.0.0/docs).\nThe sections of the prereleases since the previous stable tag are in [CHANGELOG.md at this tag](https://github.com/brunovenceslau/romeu-e-julieta/blob/v1.0.0/CHANGELOG.md).\n")
}

// TestSection reads a section by its heading: to the next level-2
// heading or the end, trailing blank lines removed; a heading inside
// an indented Why is no section start, and a missing tag is not found.
func TestSection(t *testing.T) {
	file := []byte("<!-- x -->\n\n## v1.0.0-alpha.2\n\ntwo\n\n  ## not a heading\n\n\n## v1.0.0-alpha.1\n\none\n\n")
	s, ok := Section(file, "v1.0.0-alpha.2")
	require.True(t, ok)
	assert.Equal(t, "## v1.0.0-alpha.2\n\ntwo\n\n  ## not a heading", string(s))
	s, ok = Section(file, "v1.0.0-alpha.1")
	require.True(t, ok)
	assert.Equal(t, "## v1.0.0-alpha.1\n\none", string(s))
	_, ok = Section(file, "v1.0.0")
	assert.False(t, ok)
}

// TestNormalize rewrites the seams of a prepended file and nothing
// else: the two shapes git-cliff 2.14.2 leaves (no blank line after the
// marker; two blank lines between sections and at the end) become one
// blank line each and one newline at the end, and a file that is only
// the header keeps its one newline.
func TestNormalize(t *testing.T) {
	header := "<!-- h -->\n\n" + Marker
	cases := []struct{ name, in, want string }{
		{"the first section, right after the marker", header + "\n## b\n\nx\n\n", header + "\n\n## b\n\nx\n"},
		{"two sections, two blank lines between", header + "\n\n## c\n\ny\n\n\n## b\n\nx\n\n\n", header + "\n\n## c\n\ny\n\n## b\n\nx\n"},
		{"already normal", header + "\n\n## c\n\ny\n\n## b\n\nx\n", header + "\n\n## c\n\ny\n\n## b\n\nx\n"},
		{"the header alone", header + "\n", header + "\n"},
		{"blank lines inside a section stay", header + "\n## b\n\nx\n\n\n  y\n\n", header + "\n\n## b\n\nx\n\n\n  y\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, string(normalize([]byte(tc.in))))
		})
	}
}

// TestEscape writes what is outside printable ASCII as an escape, keeps
// newlines and tabs, and cuts long output.
func TestEscape(t *testing.T) {
	assert.Equal(t, "a\tb\n\\x1b[31mc\\u00e9", escape([]byte("a\tb\n\x1b[31mcé")))
	assert.Len(t, escape(bytes.Repeat([]byte("x"), escapeMax+10)), escapeMax)
}
