// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// fixedDay is the day every test's clock shows, late in the evening of
// its zone so that a formatter in another zone would move it.
var fixedDay = time.Date(2026, time.October, 8, 23, 30, 0, 0, time.FixedZone("test", -3*60*60))

// fixture is a repository with the decision-record directory of
// .adr-dir and the records named in existing, and the env that runs
// against it.
type fixture struct {
	repo           *gittest.Repo
	e              env
	stdout, stderr *bytes.Buffer
}

func newFixture(t *testing.T, existing ...string) fixture {
	t.Helper()
	r := gittest.New(t)
	r.Write(".adr-dir", "docs/adr\n")
	for _, name := range existing {
		r.Write("docs/adr/"+name, "# x\n")
	}
	if len(existing) == 0 {
		r.Write("docs/adr/.keep", "")
	}
	f := fixture{repo: r, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	f.e = env{dir: r.Dir, gitEnv: r.Env, now: func() time.Time { return fixedDay }, stdout: f.stdout, stderr: f.stderr}
	return f
}

func (f fixture) read(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(f.repo.Dir, filepath.FromSlash(rel)))
	require.NoError(t, err)
	return string(b)
}

// TestNewADRWritesTheRecord is the acceptance case: the next number, the
// date from the injected clock, the filename from slug(title), the
// layout of rule 6 of ADR 0001 and the status Proposed.
func TestNewADRWritesTheRecord(t *testing.T) {
	f := newFixture(t, "0001-first.md", "0002-second.md")
	code := run(f.e, []string{"adr", "Adopt a new Thing, now!"})
	require.Equal(t, exitOK, code, f.stderr.String())
	assert.Equal(t, "docs/adr/0003-adopt-a-new-thing-now.md\n", f.stdout.String())
	want := "# 3. Adopt a new Thing, now!\n\nDate: 2026-10-08\n\n## Status\n\nProposed\n\n## Context\n\n## Decision\n\n## Consequences\n"
	assert.Equal(t, want, f.read(t, "docs/adr/0003-adopt-a-new-thing-now.md"))
}

// TestNewADRNumbering covers the first record, a gap (the highest number
// plus one, never a hole filled), a file that is no record, and a
// directory that looks like one.
func TestNewADRNumbering(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		want     string
	}{
		{"an empty directory starts at 1", nil, "docs/adr/0001-t.md"},
		{"the next after the highest", []string{"0001-a.md", "0002-b.md"}, "docs/adr/0003-t.md"},
		{"a gap is not filled", []string{"0001-a.md", "0005-b.md"}, "docs/adr/0006-t.md"},
		{"the index is no record", []string{"0001-a.md", "README.md"}, "docs/adr/0002-t.md"},
		{"a name without four digits is no record", []string{"0001-a.md", "12-b.md"}, "docs/adr/0002-t.md"},
		{"the number grows past two digits", []string{"0099-a.md"}, "docs/adr/0100-t.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, tt.existing...)
			require.Equal(t, exitOK, run(f.e, []string{"adr", "T"}), f.stderr.String())
			assert.Equal(t, tt.want+"\n", f.stdout.String())
		})
	}
}

// TestNewADRIgnoresADirectoryNamedLikeARecord checks that a directory
// whose name has the shape of a record does not move the number.
func TestNewADRIgnoresADirectoryNamedLikeARecord(t *testing.T) {
	f := newFixture(t, "0001-a.md")
	require.NoError(t, os.Mkdir(filepath.Join(f.repo.Dir, "docs", "adr", "0009-dir.md"), 0o755))
	require.Equal(t, exitOK, run(f.e, []string{"adr", "T"}), f.stderr.String())
	assert.Equal(t, "docs/adr/0002-t.md\n", f.stdout.String())
}

// TestNewADRDateComesFromTheClock checks that the date line is the one
// of the injected clock, in the zone the clock carries.
func TestNewADRDateComesFromTheClock(t *testing.T) {
	f := newFixture(t)
	f.e.now = func() time.Time { return time.Date(2031, time.January, 2, 0, 0, 0, 0, time.UTC) }
	require.Equal(t, exitOK, run(f.e, []string{"adr", "Dated"}), f.stderr.String())
	assert.Contains(t, f.read(t, "docs/adr/0001-dated.md"), "\nDate: 2031-01-02\n")
}

// TestNewADRFromASubdirectory checks that the directory the command
// starts in does not matter: the record lands in the one of .adr-dir.
func TestNewADRFromASubdirectory(t *testing.T) {
	f := newFixture(t, "0001-a.md")
	sub := filepath.Join(f.repo.Dir, "tools")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	f.e.dir = sub
	require.Equal(t, exitOK, run(f.e, []string{"adr", "From below"}), f.stderr.String())
	assert.FileExists(t, filepath.Join(f.repo.Dir, "docs", "adr", "0002-from-below.md"))
}

// TestNewADRFollowsAdrDir checks that the directory comes from .adr-dir
// and not from a constant.
func TestNewADRFollowsAdrDir(t *testing.T) {
	f := newFixture(t)
	f.repo.Write(".adr-dir", "decisions\n")
	require.NoError(t, os.Mkdir(filepath.Join(f.repo.Dir, "decisions"), 0o755))
	require.Equal(t, exitOK, run(f.e, []string{"adr", "Elsewhere"}), f.stderr.String())
	assert.Equal(t, "decisions/0001-elsewhere.md\n", f.stdout.String())
}

// TestNewADRRefuses covers every error: each one exits 2, says why on
// standard error and writes nothing.
func TestNewADRRefuses(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		setup func(t *testing.T, f fixture)
		want  string
	}{
		{"no title", []string{"adr"}, nil, "adr takes the title as one argument"},
		{"two arguments", []string{"adr", "a", "b"}, nil, "adr takes the title as one argument"},
		{"an empty title", []string{"adr", "  "}, nil, "the title is empty"},
		{"a line break in the title", []string{"adr", "a\nb"}, nil, "character that does not show (U+000A)"},
		{"a right-to-left override in the title", []string{"adr", "Adopt \u202eX"}, nil, "character that does not show (U+202E)"},
		{"a zero-width space in the title", []string{"adr", "Adopt\u200bX"}, nil, "character that does not show (U+200B)"},
		{"a Hangul filler in the title", []string{"adr", "Adopt\u3164X"}, nil, "character that does not show (U+3164)"},
		{"a Hangul choseong filler in the title", []string{"adr", "Adopt\u115fX"}, nil, "character that does not show (U+115F)"},
		{"a Hangul jungseong filler in the title", []string{"adr", "Adopt\u1160X"}, nil, "character that does not show (U+1160)"},
		{"a halfwidth Hangul filler in the title", []string{"adr", "Adopt\uffa0X"}, nil, "character that does not show (U+FFA0)"},
		{"an empty Braille pattern in the title", []string{"adr", "Adopt\u2800X"}, nil, "character that does not show (U+2800)"},
		{"a combining grapheme joiner in the title", []string{"adr", "Adopt\u034fX"}, nil, "character that does not show (U+034F)"},
		{"a Khmer inherent vowel in the title", []string{"adr", "Adopt\u17b4X"}, nil, "character that does not show (U+17B4)"},
		{"an unassigned default-ignorable code point in the title", []string{"adr", "Adopt\u2065X"}, nil, "character that does not show (U+2065)"},
		{"a Mongolian free variation selector in the title", []string{"adr", "Adopt\u180bX"}, nil, "character that does not show (U+180B)"},
		{"a variation selector in the title", []string{"adr", "Adopt\ufe0fX"}, nil, "character that does not show (U+FE0F)"},
		{"a supplementary variation selector in the title", []string{"adr", "Adopt\U000e0100X"}, nil, "character that does not show (U+E0100)"},
		{"a zero width joiner in the title", []string{"adr", "Adopt\u200dX"}, nil, "character that does not show (U+200D)"},
		{"a zero width non-joiner in the title", []string{"adr", "Adopt\u200cX"}, nil, "character that does not show (U+200C)"},
		{"a no-break space only", []string{"adr", "\u00a0"}, nil, "the title is empty"},
		{"spaces and a no-break space only", []string{"adr", " \u00a0\u3000 "}, nil, "the title is empty"},
		{"a line separator in the title", []string{"adr", "Adopt\u2028X"}, nil, "character that does not show (U+2028)"},
		{"a paragraph separator in the title", []string{"adr", "Adopt\u2029X"}, nil, "character that does not show (U+2029)"},
		{"an .adr-dir that links outside the repository", []string{"adr", "T"}, func(t *testing.T, f fixture) {
			require.NoError(t, os.Symlink(t.TempDir(), filepath.Join(f.repo.Dir, "out")))
			f.repo.Write(".adr-dir", "out\n")
		}, "a link leads out of the repository"},
		{"a title with no ASCII letter or digit", []string{"adr", "?!"}, nil, "filename would be empty"},
		{"an unknown kind", []string{"rule", "x"}, nil, `unknown kind "rule"`},
		{"no kind", nil, nil, "usage:"},
		{"no .adr-dir", []string{"adr", "T"}, func(t *testing.T, f fixture) {
			require.NoError(t, os.Remove(filepath.Join(f.repo.Dir, ".adr-dir")))
		}, ".adr-dir"},
		{"an .adr-dir outside the repository", []string{"adr", "T"}, func(_ *testing.T, f fixture) {
			f.repo.Write(".adr-dir", "../out\n")
		}, "not a directory inside the repository"},
		{"a missing directory", []string{"adr", "T"}, func(t *testing.T, f fixture) {
			require.NoError(t, os.RemoveAll(filepath.Join(f.repo.Dir, "docs")))
		}, "docs"},
		{"not a repository", []string{"adr", "T"}, func(t *testing.T, f fixture) {
			require.NoError(t, os.RemoveAll(filepath.Join(f.repo.Dir, ".git")))
		}, "git rev-parse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.setup != nil {
				tt.setup(t, f)
			}
			assert.Equal(t, exitError, run(f.e, tt.args))
			assert.Empty(t, f.stdout.String())
			assert.Contains(t, f.stderr.String(), tt.want)
			// The record directory is gone in two cases, and then there is
			// nothing to count.
			if entries, err := os.ReadDir(filepath.Join(f.repo.Dir, "docs", "adr")); err == nil {
				assert.Len(t, entries, 1, "only .keep: nothing was written")
			}
		})
	}
}

// TestNewADRNeverOverwrites checks that a name that exists is an error
// and leaves its bytes alone. A second session can take the same number
// between the scan and the write, so the test plants the file in the
// clock, which runs between them.
func TestNewADRNeverOverwrites(t *testing.T) {
	f := newFixture(t, "0001-a.md")
	f.e.now = func() time.Time {
		f.repo.Write("docs/adr/0002-t.md", "mine\n")
		return fixedDay
	}
	assert.Equal(t, exitError, run(f.e, []string{"adr", "T"}))
	assert.Equal(t, "mine\n", f.read(t, "docs/adr/0002-t.md"))
	assert.Contains(t, f.stderr.String(), "file exists")
}

// TestNewADRWithAMixedTitle checks a title that holds letters outside
// ASCII: they separate the words of the filename and stay in the heading.
func TestNewADRWithAMixedTitle(t *testing.T) {
	f := newFixture(t)
	require.Equal(t, exitOK, run(f.e, []string{"adr", "Decis\u00e3o sobre X"}), f.stderr.String())
	assert.Equal(t, "docs/adr/0001-decis-o-sobre-x.md\n", f.stdout.String())
	assert.True(t, strings.HasPrefix(f.read(t, "docs/adr/0001-decis-o-sobre-x.md"), "# 1. Decis\u00e3o sobre X\n\nDate: 2026-10-08\n"))
}

// TestNewADRNumberCeiling checks the number 9999: a record may take it,
// and the next one is refused, since a fifth digit would not match the
// name of a record and the numbers would start again at 10000.
func TestNewADRNumberCeiling(t *testing.T) {
	f := newFixture(t, "9998-a.md")
	require.Equal(t, exitOK, run(f.e, []string{"adr", "T"}), f.stderr.String())
	assert.Equal(t, "docs/adr/9999-t.md\n", f.stdout.String())
	f.stdout.Reset()
	assert.Equal(t, exitError, run(f.e, []string{"adr", "U"}))
	assert.Contains(t, f.stderr.String(), "last number a filename has room for")
	assert.NoFileExists(t, filepath.Join(f.repo.Dir, "docs", "adr", "10000-u.md"))
}

// TestNewADRFailsOnADuplicateNumber checks the run that loses a race: a
// second session creates a record with the same number between the scan
// and the check after the write (the clock runs between the scan and the
// create, so the test plants the other record there, under another name,
// which O_EXCL cannot see). The run removes its own file and fails, and
// the other record stays.
func TestNewADRFailsOnADuplicateNumber(t *testing.T) {
	f := newFixture(t, "0001-a.md")
	f.e.now = func() time.Time {
		f.repo.Write("docs/adr/0002-other.md", "other\n")
		return fixedDay
	}
	assert.Equal(t, exitError, run(f.e, []string{"adr", "T"}))
	assert.Contains(t, f.stderr.String(), "another record took number 0002")
	assert.Empty(t, f.stdout.String())
	assert.NoFileExists(t, filepath.Join(f.repo.Dir, "docs", "adr", "0002-t.md"))
	assert.Equal(t, "other\n", f.read(t, "docs/adr/0002-other.md"))
}

// TestNewADRStaysInsideTheRepository checks that a record directory that
// is a link to a directory outside the repository is not written to.
func TestNewADRStaysInsideTheRepository(t *testing.T) {
	f := newFixture(t)
	outside := t.TempDir()
	require.NoError(t, os.RemoveAll(filepath.Join(f.repo.Dir, "docs", "adr")))
	require.NoError(t, os.Symlink(outside, filepath.Join(f.repo.Dir, "docs", "adr")))
	assert.Equal(t, exitError, run(f.e, []string{"adr", "T"}))
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	assert.Empty(t, entries, "nothing was written outside")
}

// TestCheckTitleAccepts pins what a title may hold: accented and
// non-Latin letters, a combining accent and ordinary spaces.
func TestCheckTitleAccepts(t *testing.T) {
	for _, title := range []string{"Adopt a café rule", "Café au lait", "日本語", "A  B"} {
		assert.NoError(t, checkTitle(title), "%q", title)
	}
}
