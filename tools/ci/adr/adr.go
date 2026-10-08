// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package adr reads the decision records of docs/adr in the layout of
// ADR 0001, rule 6. It holds the one slug function that "tools/new adr"
// names a record with and that the sequences check compares filenames
// through, so the two can never disagree; it lives below tools/ci, on
// the checks surface of the ask-first list (05 5.3).
package adr

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// Dir is the directory of the records, relative to the top of the
// working tree, and Index the generated file there that is not a
// record (ADR 0001, rule 7).
const (
	Dir   = "docs/adr"
	Index = "README.md"
)

// Sections are the level-2 headings of a record, in their order
// (ADR 0001, rule 6).
var Sections = []string{"## Status", "## Context", "## Decision", "## Consequences"}

// fileName is the name of a record: four digits, a hyphen, a slug.
var fileName = regexp.MustCompile(`^([0-9]{4})-(.+)\.md$`)

// Record is what the index shows of one record.
type Record struct {
	Number int
	Title  string
	// Status is one of Statuses: "Superseded" for a record whose
	// status line is "Superseded by [...]".
	Status string
	// File is the name of the record's file in Dir.
	File string
}

// Slug lowers the ASCII letters of the title, turns each run of other
// characters into one hyphen, and trims hyphens from both ends (ADR
// 0001, rule 6). Only ASCII is lowered: a character whose Unicode lower
// case is an ASCII letter, such as the Kelvin sign, stays a separator,
// so the result never depends on the Unicode tables of a Go version.
func Slug(title string) string {
	var b strings.Builder
	gap := false
	for i := 0; i < len(title); i++ {
		c := title[i]
		switch {
		case 'A' <= c && c <= 'Z':
			c += 'a' - 'A'
		case 'a' <= c && c <= 'z', '0' <= c && c <= '9':
		default:
			gap = true
			continue
		}
		if gap && b.Len() > 0 {
			b.WriteByte('-')
		}
		gap = false
		b.WriteByte(c)
	}
	return b.String()
}

// FileName returns the name of the file of record number n with the
// title: "NNNN-" plus the slug of the title, plus ".md".
func FileName(n int, title string) string {
	return fmt.Sprintf("%04d-%s.md", n, Slug(title))
}

// Last is the highest number a record can take: the filename holds
// four digits.
const Last = 9999

// Statuses are the statuses a record can have (ADR 0001, rule 7).
var Statuses = []string{"Proposed", "Accepted", "Rejected", "Deprecated", "Superseded"}

// supersededBy starts the status line of a record that a later one
// supersedes (ADR 0001, rule 7).
const supersededBy = "Superseded by "

// named is a file of dir named as a record.
type named struct {
	number int
	name   string
}

// recordFiles returns the entries of dir in fsys named as records. Next
// and Read both read through it, so they agree on what a record is; an
// entry named as a record that is not a regular file, such as a
// directory or a symbolic link, is refused by both. dir is a path of
// fsys: slash-separated, without a leading slash.
func recordFiles(fsys fs.FS, dir string) ([]named, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	var files []named
	for _, e := range entries {
		m := fileName.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		if !e.Type().IsRegular() {
			return nil, fmt.Errorf("%s is named as a record and is not a file", e.Name())
		}
		n, _ := strconv.Atoi(m[1]) // four digits always parse
		files = append(files, named{n, e.Name()})
	}
	return files, nil
}

// Next returns the number the next record takes in dir of fsys: one
// past the highest number of a file there named as a record, or 1 when
// there is none. A gap before it is the sequences check's to report. It
// fails when the highest number is Last.
func Next(fsys fs.FS, dir string) (int, error) {
	files, err := recordFiles(fsys, dir)
	if err != nil {
		return 0, err
	}
	highest := 0
	for _, f := range files {
		highest = max(highest, f.number)
	}
	if highest == Last {
		return 0, fmt.Errorf("record %d exists, and a filename holds four digits: no number is left", Last)
	}
	return highest + 1, nil
}

// Read returns the records of dir in fsys, the files named as records,
// by number. fsys is the working tree when a record is written, and the
// tree of a commit when a check judges one. It reads what the index
// needs, the title line and the status, and refuses a record that lacks
// one of them, a status outside Statuses, or a number that two records
// hold. Every other rule of the layout is the sequences check's.
func Read(fsys fs.FS, dir string) ([]Record, error) {
	files, err := recordFiles(fsys, dir)
	if err != nil {
		return nil, err
	}
	var records []Record
	for _, f := range files {
		data, err := fs.ReadFile(fsys, path.Join(dir, f.name))
		if err != nil {
			return nil, err
		}
		r, err := parse(f.number, string(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.name, err)
		}
		r.File = f.name
		if i := slices.IndexFunc(records, func(o Record) bool { return o.Number == f.number }); i >= 0 {
			return nil, fmt.Errorf("two records hold the number %d: %s and %s", f.number, records[i].File, r.File)
		}
		records = append(records, r)
	}
	slices.SortFunc(records, func(a, b Record) int { return a.Number - b.Number })
	return records, nil
}

// parse reads the title line and the status of record number n. A line
// is read without a trailing carriage return, and a heading without
// trailing white space. The status is the first line of the Status
// section that is not blank: one of Statuses, or "Superseded by" and
// the link to the record that supersedes it.
func parse(n int, text string) (Record, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	prefix := "# " + strconv.Itoa(n) + ". "
	title, ok := strings.CutPrefix(lines[0], prefix)
	if !ok || strings.TrimSpace(title) == "" {
		return Record{}, fmt.Errorf("the first line is not %q", prefix+"<title>")
	}
	r := Record{Number: n, Title: title}
	start := slices.IndexFunc(lines, func(l string) bool { return strings.TrimRightFunc(l, unicode.IsSpace) == Sections[0] })
	if start < 0 {
		return Record{}, errors.New(`no "` + Sections[0] + `" section`)
	}
	for _, line := range lines[start+1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			break
		}
		switch {
		case slices.Contains(Statuses, line):
			r.Status = line
		case strings.HasPrefix(line, supersededBy):
			r.Status = "Superseded"
		default:
			return Record{}, fmt.Errorf("the status %q is not one of %s, or %q and a link", line, strings.Join(Statuses, ", "), strings.TrimSpace(supersededBy))
		}
		return r, nil
	}
	return Record{}, errors.New(`no line under "` + Sections[0] + `"`)
}
