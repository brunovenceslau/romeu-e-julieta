// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/adrdir"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/slug"
)

func init() {
	register(kind{name: "adr", args: "<title>", run: newADR})
}

// newADR writes the next decision record, with the layout of rule 6 of
// ADR 0001: the title line, the date from the clock, and the four
// sections, with the status Proposed. The file is created and never
// overwritten, so a name that exists is an error. It reads and writes
// through the root of the repository, so a symbolic link cannot send it
// outside. Two runs that take the same number at once each create their
// own file and then scan again: the run whose re-scan sees two files
// with its number removes its own and fails, so at most one record keeps
// the number.
func newADR(e env, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("adr takes the title as one argument\n%s", usage())
	}
	title := strings.TrimSpace(args[0])
	if err := checkTitle(title); err != nil {
		return err
	}
	name := slug.Slug(title)
	if name == "" {
		return fmt.Errorf("the title %q has no ASCII letter or digit, so its filename would be empty", title)
	}
	top, err := repoRoot(e)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(top)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	rel, err := adrdir.Dir(root)
	if err != nil {
		return err
	}
	byNumber, err := recordsByNumber(root, rel)
	if err != nil {
		return err
	}
	n := nextNumber(byNumber)
	if n > adrdir.MaxNumber {
		return fmt.Errorf("%s holds record %04d, the last number a filename has room for", rel, adrdir.MaxNumber)
	}
	file := fmt.Sprintf("%04d-%s.md", n, name)
	path := filepath.Join(rel, file)
	body := fmt.Sprintf("# %d. %s\n\nDate: %s\n\n## Status\n\nProposed\n\n## Context\n\n## Decision\n\n## Consequences\n",
		n, title, e.now().Format(time.DateOnly))
	if err := create(root, path, body); err != nil {
		return err
	}
	after, err := recordsByNumber(root, rel)
	if err == nil && len(after[n]) > 1 {
		err = fmt.Errorf("another record took number %04d at the same time (%s); run the command again", n, strings.Join(after[n], ", "))
	}
	if err != nil {
		// Not ours to keep: the number is not unique, or not known to be.
		if rmErr := root.Remove(path); rmErr != nil {
			err = errors.Join(err, rmErr)
		}
		return err
	}
	say(e.stdout, "%s\n", filepath.ToSlash(path))
	return nil
}

// recordsByNumber maps each record number in the directory rel below
// root to the filenames that carry it. A directory with a record's name
// is not a record.
func recordsByNumber(root *os.Root, rel string) (map[int][]string, error) {
	entries, err := fs.ReadDir(root.FS(), filepath.ToSlash(rel))
	if err != nil {
		return nil, err
	}
	byNumber := map[int][]string{}
	for _, entry := range entries {
		m := adrdir.Name.FindStringSubmatch(entry.Name())
		if m == nil || entry.Type()&fs.ModeDir != 0 {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, err
		}
		byNumber[n] = append(byNumber[n], entry.Name())
	}
	return byNumber, nil
}

// nextNumber returns one more than the highest number of the records,
// and 1 when there are none.
func nextNumber(byNumber map[int][]string) int {
	highest := 0
	for n := range byNumber {
		highest = max(highest, n)
	}
	return highest + 1
}
