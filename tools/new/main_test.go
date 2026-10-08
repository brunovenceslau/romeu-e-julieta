// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/adr"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// clock is the injected clock of every test (10 10.4): early on
// 2026-10-08 in a zone two hours east of UTC, where UTC still reads
// 2026-10-07. The record takes the date of the clock's own zone, the
// day the author sees, so a date read in UTC fails the test.
var clock = func() time.Time {
	return time.Date(2026, 10, 8, 1, 30, 0, 0, time.FixedZone("east", 2*60*60))
}

// newRepo returns a fixture repository with two records in docs/adr.
func newRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	r.Write("docs/adr/0001-pick-a-thing.md", "# 1. Pick a thing\n\n## Status\n\nAccepted\n")
	r.Write("docs/adr/0002-pick-another.md", "# 2. Pick another\n\n## Status\n\nAccepted\n")
	r.Write("docs/adr/README.md", "the index\n")
	return r
}

// runNew runs the command in dir, with the fixture's git environment,
// and returns its exit status and what it printed.
func runNew(t *testing.T, r *gittest.Repo, dir string, args ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	e := env{repo: git.Repo{Dir: dir, Env: r.Env}, now: clock, link: os.Link, stdout: &out, stderr: &out}
	return run(t.Context(), e, args), out.String()
}

const wantRecord = `# 3. Use a plain name, twice!

Date: 2026-10-08

## Status

Proposed

## Context

What makes this decision necessary now, and what we measured before
making it.

### Alternatives considered

- **The first alternative.** What it would have given us, and why we
  did not choose it.

## Decision

We will ...

## Consequences

What becomes easier, what becomes harder, and what would make us
revisit this decision.
`

// TestADR covers "tools/new adr <title>" (12 12.3; ADR 0001, rule 6):
// the next number, the date from the injected clock, the title line,
// the filename from the slug of the title, and status Proposed. It runs
// from a subdirectory, as "go run" from anywhere in the tree does.
func TestADR(t *testing.T) {
	r := newRepo(t)
	sub := filepath.Join(r.Dir, "docs")
	code, out := runNew(t, r, sub, "adr", "Use a plain name, twice!")
	require.Equal(t, exitOK, code, out)
	assert.Equal(t, "wrote docs/adr/0003-use-a-plain-name-twice.md\nrun \"go generate ./...\" to add it to the index, docs/adr/README.md\n", out)
	got, err := os.ReadFile(filepath.Join(r.Dir, "docs", "adr", "0003-use-a-plain-name-twice.md"))
	require.NoError(t, err)
	assert.Equal(t, wantRecord, string(got))

	records, err := adr.Read(os.DirFS(r.Dir), adr.Dir)
	require.NoError(t, err)
	require.Len(t, records, 3, "the new record reads as one")
	assert.Equal(t, adr.Record{Number: 3, Title: "Use a plain name, twice!", Status: "Proposed", File: "0003-use-a-plain-name-twice.md"}, records[2])

	code, out = runNew(t, r, r.Dir, "adr", "Pick a fourth")
	require.Equal(t, exitOK, code, out)
	assert.Contains(t, out, "wrote docs/adr/0004-pick-a-fourth.md\n", "the number after the one just written")

	code, out = runNew(t, r, r.Dir, "adr", "Usar a ação rápida")
	require.Equal(t, exitOK, code, out)
	assert.Contains(t, out, "wrote docs/adr/0005-usar-a-a-o-r-pida.md\n", "the filename keeps ASCII only")
	got, err = os.ReadFile(filepath.Join(r.Dir, "docs", "adr", "0005-usar-a-a-o-r-pida.md"))
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(got), "# 5. Usar a ação rápida\n"), "the title line keeps the title as typed")
}

// TestExitCodes pins the command's exit statuses, the contract a
// script that calls it reads.
func TestExitCodes(t *testing.T) {
	assert.Equal(t, 0, exitOK)
	assert.Equal(t, 1, exitFail)
	assert.Equal(t, 2, exitUsage)
}

// newFileMode returns the mode a file created with 0666 takes in dir:
// 0666 less the umask of this process.
func newFileMode(t *testing.T, dir string) fs.FileMode {
	t.Helper()
	ref := filepath.Join(dir, "reference")
	f, err := os.OpenFile(ref, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	info, err := os.Stat(ref)
	require.NoError(t, err)
	require.NoError(t, os.Remove(ref))
	return info.Mode().Perm()
}

// assertOnly checks that dir holds exactly the files named, so that no
// temporary file stays behind.
func assertOnly(t *testing.T, dir string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	assert.ElementsMatch(t, names, got, "no temporary file stays behind")
}

// TestCreateNew covers the one write of the command: the file takes
// mode 0666 less the umask, a name that exists is refused, left as it
// was and named in the error rather than the temporary file, a write
// that cannot start leaves nothing, and no temporary file stays behind.
func TestCreateNew(t *testing.T) {
	dir := t.TempDir()
	want := newFileMode(t, dir)
	path := filepath.Join(dir, "0001-a.md")
	require.NoError(t, createNew(path, []byte("first\n"), os.Link))
	err := createNew(path, []byte("second\n"), os.Link)
	require.ErrorIs(t, err, fs.ErrExist)
	assert.Equal(t, path+" exists, and a record is never replaced: file already exists", err.Error())
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "first\n", string(got), "the file that was there is untouched")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, want, info.Mode().Perm())
	assertOnly(t, dir, "0001-a.md")

	missing := filepath.Join(dir, "missing", "0002-b.md")
	require.Error(t, createNew(missing, []byte("x\n"), os.Link))
	assert.NoFileExists(t, missing)
}

// TestCreateNewWithoutHardLinks covers a file system without hard
// links, where link fails with an error other than fs.ErrExist: the
// file is written through an exclusive create, which refuses a name
// that exists too, and the temporary file is removed either way.
func TestCreateNewWithoutHardLinks(t *testing.T) {
	var linked []string
	noLinks := func(oldname, newname string) error {
		linked = append(linked, oldname)
		return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: errors.ErrUnsupported}
	}
	dir := t.TempDir()
	want := newFileMode(t, dir)
	path := filepath.Join(dir, "0001-a.md")
	require.NoError(t, createNew(path, []byte("first\n"), noLinks))
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "first\n", string(got))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, want, info.Mode().Perm())
	require.Len(t, linked, 1)
	assert.True(t, strings.HasPrefix(filepath.Base(linked[0]), ".new-"), "link was asked to name the temporary file")
	assertOnly(t, dir, "0001-a.md")

	err = createNew(path, []byte("second\n"), noLinks)
	require.ErrorIs(t, err, fs.ErrExist)
	assert.NotContains(t, err.Error(), ".new-", "the error names the record, not the temporary file")
	got, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "first\n", string(got), "the file that was there is untouched")
	assertOnly(t, dir, "0001-a.md")
}

// TestCreateNewLinkFails covers a link that fails, and a fallback
// create that fails as well: the error says why, and neither the
// record nor the temporary file is left.
func TestCreateNewLinkFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0001-a.md")
	// The link fails, and the exclusive create of the fallback fails
	// too: a directory has the name by then.
	failing := func(_, newname string) error {
		require.NoError(t, os.Mkdir(newname, 0o755))
		return syscall.EPERM
	}
	err := createNew(path, []byte("x\n"), failing)
	require.ErrorIs(t, err, fs.ErrExist)
	assert.Contains(t, err.Error(), path+" exists")
	assertOnly(t, dir, "0001-a.md")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "what was at the name is untouched")
}

// TestCreateNewConcurrently runs two creates of one name at once, as
// two runs of "tools/new adr" that pick the same number: one writes the
// record whole, the other fails with fs.ErrExist, and no temporary
// file stays behind.
func TestCreateNewConcurrently(t *testing.T) {
	for range 20 {
		dir := t.TempDir()
		path := filepath.Join(dir, "0001-a.md")
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i := range errs {
			wg.Go(func() { errs[i] = createNew(path, []byte(strings.Repeat(strconv.Itoa(i), 4096)), os.Link) })
		}
		wg.Wait()
		winner := slices.IndexFunc(errs, func(err error) bool { return err == nil })
		require.GreaterOrEqual(t, winner, 0, "one create succeeds: %v", errs)
		require.ErrorIs(t, errs[1-winner], fs.ErrExist)
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, strings.Repeat(strconv.Itoa(winner), 4096), string(got), "the record is the winner's, whole")
		assertOnly(t, dir, "0001-a.md")
	}
}

// TestADRRefuses covers the titles and the trees "tools/new adr"
// refuses, and checks that it writes nothing then.
func TestADRRefuses(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		code  int
		want  string
		setup func(t *testing.T, r *gittest.Repo)
	}{
		{name: "no kind", args: nil, code: exitUsage, want: "usage:"},
		{name: "an unknown kind", args: []string{"lesson", "x"}, code: exitUsage, want: `unknown kind "lesson"`},
		{name: "no title", args: []string{"adr"}, code: exitUsage, want: "adr takes one argument, the title"},
		{name: "a title in two arguments", args: []string{"adr", "Pick", "this"}, code: exitUsage, want: "adr takes one argument, the title"},
		{name: "a title without a letter or a digit", args: []string{"adr", "--"}, code: exitFail, want: "the title needs an ASCII letter or digit"},
		{name: "a title on two lines", args: []string{"adr", "Pick\nthis"}, code: exitFail, want: "the title is one line of printable characters"},
		{name: "a title with a tab", args: []string{"adr", "Pick\tthis"}, code: exitFail, want: "the title is one line of printable characters"},
		{name: "a title with space around it", args: []string{"adr", " Pick this"}, code: exitFail, want: "the title has no space at either end"},
		{name: "a title too long for a filename", args: []string{"adr", strings.Repeat("a", 250)}, code: exitFail, want: "would be 258 bytes, and a file system takes 255 at most"},
		{
			name: "no record directory", args: []string{"adr", "Pick"}, code: exitFail, want: "docs/adr",
			setup: func(t *testing.T, r *gittest.Repo) { require.NoError(t, os.RemoveAll(filepath.Join(r.Dir, "docs"))) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRepo(t)
			if tt.setup != nil {
				tt.setup(t, r)
			}
			before := r.Git("status", "--porcelain", "--untracked-files=all")
			code, out := runNew(t, r, r.Dir, tt.args...)
			assert.Equal(t, tt.code, code, out)
			assert.Contains(t, out, tt.want)
			assert.Equal(t, before, r.Git("status", "--porcelain", "--untracked-files=all"), "nothing written")
		})
	}
}

// TestOutsideARepository checks that the command needs a working tree
// to find docs/adr in.
func TestOutsideARepository(t *testing.T) {
	r := &gittest.Repo{}
	r.Dir = t.TempDir()
	// The ceiling keeps git from finding a repository above the
	// temporary directory, when TMPDIR is inside a working tree.
	r.Env = append(gittest.Env(t), "GIT_CEILING_DIRECTORIES="+filepath.Dir(r.Dir))
	code, out := runNew(t, r, r.Dir, "adr", "Pick")
	assert.Equal(t, exitFail, code, out)
	assert.Contains(t, out, "new: git rev-parse")
}
