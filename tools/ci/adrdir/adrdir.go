// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package adrdir holds what the author of a decision record (tools/new)
// and the generator of its index (tools/ci generate) both need to find
// the records: where the directory is, and what a record's filename
// looks like (rule 6 of ADR 0001). One copy of each, so they cannot
// disagree about which files are records.
package adrdir

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// File names, relative to the repository root, the file that says where
// the decision records live.
const File = ".adr-dir"

// MaxNumber is the highest record number: the filename holds four
// digits, so a fifth would not match Name and the numbering would
// start again.
const MaxNumber = 9999

// Name matches the filename of a decision record: four digits, a
// hyphen, and the rest. The first submatch is the number.
var Name = regexp.MustCompile(`^(\d{4})-.+\.md$`)

// Dir returns the directory that .adr-dir names, cleaned, relative to
// root. The file is read through root, and the directory must lie inside
// it and outside .git, by the path it names and by the path that its
// symbolic links lead to.
func Dir(root *os.Root) (string, error) {
	data, err := root.ReadFile(File)
	if err != nil {
		return "", err
	}
	named := strings.TrimSpace(string(data))
	if !filepath.IsLocal(named) {
		return "", fmt.Errorf("%s names %q, which is not a directory inside the repository", File, named)
	}
	// Clean, so that "docs/adr/" and "./docs/adr" read as the directory
	// they name (fs.ReadDir takes neither), and so that the first
	// element is the real one.
	dir := filepath.Clean(named)
	// The name as written first: it is refused with this message even when
	// a link below .git would make resolve fail with another.
	if insideGit(dir) {
		return "", fmt.Errorf("%s names %q, which is inside the .git directory, not a place for records", File, named)
	}
	leads, err := resolve(root, dir)
	if err != nil {
		return "", fmt.Errorf("%s names %q: %w", File, named, err)
	}
	if insideGit(leads) {
		return "", fmt.Errorf("%s names %q, which leads through a link to %q inside the .git directory, not a place for records", File, named, leads)
	}
	return dir, nil
}

// insideGit reports whether the clean relative path has a .git component
// at any depth, as the scan of the directives leaves out a .git
// directory wherever it stands.
func insideGit(dir string) bool {
	return slices.ContainsFunc(strings.Split(filepath.ToSlash(dir), "/"), func(c string) bool { return strings.EqualFold(c, ".git") })
}

// maxLinks bounds the links followed by resolve, as the kernel bounds
// them, so that a loop of links is an error and not a hang.
const maxLinks = 40

// resolve returns the path that rel leads to below root once every
// symbolic link in it is followed, one component at a time. A link that
// leaves root, whether by an absolute target or by climbing out, is an
// error. A component that does not exist ends the following: the rest
// of the path is kept as written, so a dangling link resolves to the
// place it points at.
func resolve(root *os.Root, rel string) (string, error) {
	pending := strings.Split(filepath.ToSlash(rel), "/")
	var done []string
	links := 0
	for len(pending) > 0 {
		c := pending[0]
		pending = pending[1:]
		switch c {
		case "", ".":
			continue
		case "..":
			if len(done) == 0 {
				return "", errors.New("a link leads out of the repository")
			}
			done = done[:len(done)-1]
			continue
		}
		cand := path.Join(strings.Join(done, "/"), c)
		info, err := root.Lstat(cand)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			done = append(done, c)
		case err != nil:
			return "", err
		case info.Mode()&fs.ModeSymlink != 0:
			if links++; links > maxLinks {
				return "", errors.New("too many links")
			}
			target, err := root.Readlink(cand)
			if err != nil {
				return "", err
			}
			if filepath.IsAbs(target) {
				return "", errors.New("a link leads out of the repository")
			}
			pending = append(strings.Split(filepath.ToSlash(target), "/"), pending...)
		default:
			done = append(done, c)
		}
	}
	return strings.Join(done, "/"), nil
}
