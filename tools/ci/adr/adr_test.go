// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package adr

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSlug pins slug(title) of ADR 0001, rule 6: ASCII letters lowered,
// each run of other characters one hyphen, no hyphen at either end.
// Only ASCII is lowered, so a character whose Unicode lower case is an
// ASCII letter (the Kelvin sign) is a separator like any other.
func TestSlug(t *testing.T) {
	tests := []struct {
		title, want string
	}{
		{"Adopt testify assert and require in tests", "adopt-testify-assert-and-require-in-tests"},
		{"Let the sandbox act as the maintainer on GitHub", "let-the-sandbox-act-as-the-maintainer-on-github"},
		{"Use v1.2 -- twice!", "use-v1-2-twice"},
		{"  Trim (both) ends.  ", "trim-both-ends"},
		{"Café au lait", "caf-au-lait"},
		{"Kelvin and İstanbul", "elvin-and-stanbul"},
		{"MiXeD 42 Case", "mixed-42-case"},
		{"Zebra Zulu 9 z", "zebra-zulu-9-z"},
		{"---", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			assert.Equal(t, tt.want, Slug(tt.title))
		})
	}
}

// TestSlugMatchesTheRecords checks slug against every record of this
// repository: each filename is its number and the slug of its title.
func TestSlugMatchesTheRecords(t *testing.T) {
	records, err := Read(os.DirFS(filepath.Join("..", "..", "..")), Dir)
	require.NoError(t, err)
	require.NotEmpty(t, records)
	for _, r := range records {
		assert.Equal(t, FileName(r.Number, r.Title), r.File)
	}
}

func TestFileName(t *testing.T) {
	assert.Equal(t, "0009-pick-a-name.md", FileName(9, "Pick a name"))
	assert.Equal(t, "1234-x.md", FileName(1234, "X"))
}

// record writes one record file into dir.
func record(t *testing.T, dir, name, text string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644))
}

const twoRecords = `# 2. Pick the second thing

Date: 2026-01-02

## Status

Superseded by [3. Pick a third thing](0003-pick-a-third-thing.md)

## Context
`

func TestRead(t *testing.T) {
	dir := t.TempDir()
	record(t, dir, "0001-pick-a-thing.md", "# 1. Pick a thing\n\nDate: 2026-01-01\n\n## Status\n\nAccepted\n\nSupersedes nothing.\n\n## Context\n\nText.\n")
	record(t, dir, "0002-pick-the-second-thing.md", twoRecords)
	record(t, dir, "0010-crlf.md", "# 10. CRLF\r\n\r\nDate: 2026-01-01\r\n\r\n## Status\r\n\r\nProposed\r\n")
	record(t, dir, "0011-spaces.md", "# 11. Spaces\n\n## Status \t\n\n  Rejected  \n\nWhy it was declined.\n")
	record(t, dir, "0012-gone.md", "# 12. Gone\n\n## Status\n\nDeprecated\n")
	record(t, dir, Index, "the index, not a record\n")
	record(t, dir, "notes.txt", "not a record\n")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "drafts"), 0o755))

	got, err := Read(os.DirFS(dir), ".")
	require.NoError(t, err)
	assert.Equal(t, []Record{
		{Number: 1, Title: "Pick a thing", Status: "Accepted", File: "0001-pick-a-thing.md"},
		{Number: 2, Title: "Pick the second thing", Status: "Superseded", File: "0002-pick-the-second-thing.md"},
		{Number: 10, Title: "CRLF", Status: "Proposed", File: "0010-crlf.md"},
		{Number: 11, Title: "Spaces", Status: "Rejected", File: "0011-spaces.md"},
		{Number: 12, Title: "Gone", Status: "Deprecated", File: "0012-gone.md"},
	}, got)
}

// TestReadRefuses covers the records Read cannot index: each error
// names the file.
func TestReadRefuses(t *testing.T) {
	valid := "# 1. A\n\n## Status\n\nAccepted\n"
	tests := []struct {
		name, file, text, want string
		pre                    map[string]string // other records, by file name
	}{
		{name: "no title line", file: "0001-a.md", text: "Date: 2026-01-01\n", want: "0001-a.md: the first line is not \"# 1. <title>\""},
		{name: "a number that differs from the filename", file: "0001-a.md", text: "# 2. A\n", want: "0001-a.md: the first line is not \"# 1. <title>\""},
		{name: "a title line with leading zeros", file: "0001-a.md", text: "# 0001. A\n", want: "the first line is not \"# 1. <title>\""},
		{name: "an empty title", file: "0001-a.md", text: "# 1. \n", want: "the first line is not \"# 1. <title>\""},
		{name: "no Status section", file: "0001-a.md", text: "# 1. A\n\n## Context\n\nAccepted\n", want: "0001-a.md: no \"## Status\" section"},
		{name: "an empty Status section", file: "0001-a.md", text: "# 1. A\n\n## Status\n\n## Context\n", want: "0001-a.md: no line under \"## Status\""},
		{name: "a status outside the set", file: "0001-a.md", text: "# 1. A\n\n## Status\n\nDraft\n", want: "0001-a.md: the status \"Draft\" is not one of Proposed, Accepted, Rejected, Deprecated, Superseded"},
		{name: "a status in another case", file: "0001-a.md", text: "# 1. A\n\n## Status\n\naccepted\n", want: "the status \"accepted\""},
		{name: "a status with more words", file: "0001-a.md", text: "# 1. A\n\n## Status\n\nAccepted today\n", want: "the status \"Accepted today\""},
		{name: "two records with one number", file: "0001-b.md", text: "# 1. B\n\n## Status\n\nAccepted\n", want: "two records hold the number 1", pre: map[string]string{"0001-a.md": valid}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, text := range tt.pre {
				record(t, dir, name, text)
			}
			record(t, dir, tt.file, tt.text)
			_, err := Read(os.DirFS(dir), ".")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
	_, err := Read(os.DirFS(t.TempDir()), "missing")
	require.Error(t, err, "a directory that does not exist")
}

func TestNext(t *testing.T) {
	dir := t.TempDir()
	n, err := Next(os.DirFS(dir), ".")
	require.NoError(t, err)
	assert.Equal(t, 1, n, "the first record of an empty directory")

	record(t, dir, "0001-a.md", "")
	record(t, dir, "0007-g.md", "") // a gap is the sequences check's to report
	record(t, dir, Index, "")
	record(t, dir, "9999.md", "")
	n, err = Next(os.DirFS(dir), ".")
	require.NoError(t, err)
	assert.Equal(t, 8, n, "one past the highest number")

	_, err = Next(os.DirFS(dir), "missing")
	require.Error(t, err)

	record(t, dir, "9999-last.md", "")
	_, err = Next(os.DirFS(dir), ".")
	require.Error(t, err, "no number after 9999")
	assert.Contains(t, err.Error(), "no number is left")
}

// TestDirectoryNamedAsARecord checks that Next and Read agree on an
// entry named as a record that is not a file: both refuse it.
func TestDirectoryNamedAsARecord(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "0003-a-directory.md"), 0o755))
	_, err := Next(os.DirFS(dir), ".")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "0003-a-directory.md is named as a record and is not a file")
	_, err = Read(os.DirFS(dir), ".")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "0003-a-directory.md is named as a record and is not a file")
}

// TestSymlinkNamedAsARecord checks that a symbolic link named as a
// record is refused, in a working tree and in the tree of a commit
// (mode 120000), even when its target is a valid record.
func TestSymlinkNamedAsARecord(t *testing.T) {
	const valid = "# 1. A\n\n## Status\n\nAccepted\n"
	dir := t.TempDir()
	record(t, dir, "target.txt", valid)
	require.NoError(t, os.Symlink("target.txt", filepath.Join(dir, "0001-a.md")))
	commit := fstest.MapFS{
		"docs/adr/target.txt": {Data: []byte(valid)},
		"docs/adr/0001-a.md":  {Data: []byte("target.txt"), Mode: fs.ModeSymlink},
	}
	for name, fsys := range map[string]fs.FS{"working tree": os.DirFS(dir), "commit": commit} {
		root := "."
		if name == "commit" {
			root = Dir
		}
		_, err := Read(fsys, root)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), "0001-a.md is named as a record and is not a file", name)
		_, err = Next(fsys, root)
		require.Error(t, err, name)
	}
}

// unreadable is a file system whose files named in fail cannot be
// read, as a record whose permissions deny the reader.
type unreadable struct {
	fs.FS
	fail string
}

func (u unreadable) Open(name string) (fs.File, error) {
	if name == u.fail {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return u.FS.Open(name)
}

// TestReadUnreadableRecord checks that a record that cannot be read is
// an error that says why, never a record left out of the index.
func TestReadUnreadableRecord(t *testing.T) {
	fsys := unreadable{fstest.MapFS{"0001-a.md": {Data: []byte("# 1. A\n\n## Status\n\nAccepted\n")}}, "0001-a.md"}
	_, err := Read(fsys, ".")
	require.ErrorIs(t, err, fs.ErrPermission)
}
