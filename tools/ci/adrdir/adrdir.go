// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package adrdir holds what the author of a decision record (tools/new)
// and the generator of its index (tools/ci generate) both need to find
// the records: where the directory is, and what a record's filename
// looks like (rule 6 of ADR 0001). One copy of each, so they cannot
// disagree about which files are records.
package adrdir

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

// Dir returns the directory that .adr-dir names, relative to root. The
// file is read through root, and the directory must lie inside it.
func Dir(root *os.Root) (string, error) {
	data, err := root.ReadFile(File)
	if err != nil {
		return "", err
	}
	dir := strings.TrimSpace(string(data))
	if !filepath.IsLocal(dir) {
		return "", fmt.Errorf("%s names %q, which is not a directory inside the repository", File, dir)
	}
	return dir, nil
}
