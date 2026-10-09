// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// generateDirective is the one go:generate directive of the repository
// (12 12.3, 10 10.2), and generateSite the file it stands in. "go
// generate ./..." runs every such line, with no sandbox, so a second one
// anywhere would run under the "generated" step: only the site may hold
// one, and only this text, once.
const (
	generateSite      = "tools/ci/generate.go"
	generateDirective = "//go:generate go run . generate"
)

// isGoGenerate is the predicate of "go generate" for a line
// (isGoGenerate in cmd/go/internal/generate/generate.go, Go 1.27): the
// line starts with the directive and a space or a tab. go generate reads
// raw lines, not comments, so the line counts wherever it stands in the
// file: in a raw string, in a block comment, in a comment.
func isGoGenerate(line []byte) bool {
	return bytes.HasPrefix(line, []byte("//go:generate ")) || bytes.HasPrefix(line, []byte("//go:generate\t"))
}

// generateLines returns each line of src that go generate would run,
// with one carriage return and then the spaces and tabs trimmed from its
// end, as go generate's split does (cmd/go/internal/generate): a form
// feed or a vertical tab stays, because it ends up inside the last word
// of the command.
func generateLines(src []byte) []string {
	var found []string
	for line := range bytes.SplitSeq(src, []byte("\n")) {
		if isGoGenerate(line) {
			found = append(found, strings.TrimRight(strings.TrimSuffix(string(line), "\r"), " \t"))
		}
	}
	return found
}

// generateProblems judges the Go files of a tree, by slash-separated
// path, and returns what is wrong with their go:generate lines: the
// repository has exactly one, with exactly the text of
// generateDirective, in generateSite.
func generateProblems(files map[string][]byte) []finding {
	var problems []finding
	site := 0
	for _, path := range slices.Sorted(maps.Keys(files)) {
		for _, line := range generateLines(files[path]) {
			switch {
			case path != generateSite:
				problems = append(problems, finding{path, "generate-directive", fmt.Sprintf("holds %q, and only %s may hold a go:generate line", line, generateSite)})
			case line != generateDirective:
				problems = append(problems, finding{path, "generate-directive", fmt.Sprintf("holds %q, and the one go:generate line of the repository is %q", line, generateDirective)})
			default:
				site++
			}
		}
	}
	if site != 1 {
		problems = append(problems, finding{generateSite, "generate-directive", fmt.Sprintf("holds the go:generate line %q %d times, and it must hold it once", generateDirective, site)})
	}
	return problems
}

// checkGenerateDirectives reads every .go file below root and returns
// generateProblems for them. It is a superset of what "go generate
// ./..." reads: it takes no account of build constraints, file-name
// suffixes, testdata, vendor, "_" and "." directories or nested
// modules, and it reads a symbolic link to a file as the file. It leaves
// out a directory named .git at any depth, a nested one included, a
// symbolic link to a directory (it does not enter the target; a target
// inside the tree is read once, through its real path) and a dangling
// link. A link to a directory whose name ends in .go cannot be read as a
// file, and that is an error, not a pass.
func checkGenerateDirectives(root string) ([]finding, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && d.Name() == ".git":
			return filepath.SkipDir
		case d.IsDir() || filepath.Ext(path) != ".go":
			return nil
		}
		src, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			return nil // a dangling link: nothing for go generate to read
		}
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = src
		return nil
	})
	if err != nil {
		return nil, err
	}
	return generateProblems(files), nil
}
