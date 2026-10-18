// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

func init() {
	register(kind{name: "error", args: "<package> <slug>", run: newError})
}

// The shapes the error kind accepts: a package is a path below
// internal/ of lowercase words, and a slug is lowercase words joined by
// single dashes (04 4.1, Error ids).
var (
	packageWord = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
	errorSlug   = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
)

// errorTable is the page that holds the list of 04 4.4, which stands
// for the rows no package declares yet.
const errorTable = "docs/spec/04-cli.md"

// cliImport is the import path of the package that declares Row.
const cliImport = "github.com/brunovenceslau/romeu-e-julieta/internal/cli"

// newError declares the next id of the error table (12 12.3, REQ-1127;
// 04 4.1, Error ids, item 3) in the errors.go of the package named,
// internal/<package>/errors.go, which it creates when it is absent: the
// number after the highest one that a package declares or the list of
// 04 4.4 holds, the slug, and the exit code, the hint and the "recovered
// in" cell to fill. The union test of the table fails until they are
// filled and the row joins 04 4.4.
func newError(e env, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("error takes a package below internal/ and a slug\n%s", usage())
	}
	pkg, slug := args[0], args[1]
	for w := range strings.SplitSeq(pkg, "/") {
		if !packageWord.MatchString(w) {
			return fmt.Errorf("the package %q is not a path below internal/ of lowercase words", pkg)
		}
		if token.IsKeyword(w) {
			return fmt.Errorf("the package %q holds the Go keyword %q", pkg, w)
		}
	}
	if !errorSlug.MatchString(slug) {
		return fmt.Errorf("the slug %q is not lowercase words joined by single dashes", slug)
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
	dir := path.Join("internal", pkg)
	if info, err := fs.Stat(root.FS(), dir); err != nil || !info.IsDir() {
		return fmt.Errorf("%s is no package directory", dir)
	}
	declared, err := cli.ReadTable(root.FS())
	if err != nil {
		return err
	}
	spec, err := fs.ReadFile(root.FS(), errorTable)
	if err != nil {
		return err
	}
	listed, err := cli.SpecRows(spec)
	if err != nil {
		return err
	}
	highest := 0
	seen := func(id, s string) error {
		if s == slug {
			return fmt.Errorf("the slug %q is the error %s already", slug, id)
		}
		n, err := strconv.Atoi(strings.TrimPrefix(id, "RJ-"))
		if err != nil {
			return fmt.Errorf("the id %q is not RJ-<nnn>", id)
		}
		highest = max(highest, n)
		return nil
	}
	for _, d := range declared {
		if err := seen(d.ID, d.Slug); err != nil {
			return err
		}
	}
	for _, r := range listed {
		if err := seen(r.ID, r.Slug); err != nil {
			return err
		}
	}
	if highest >= 999 {
		return fmt.Errorf("the table holds RJ-%03d, the last id of three digits", highest)
	}
	id := fmt.Sprintf("RJ-%03d", highest+1)
	if e.afterScan != nil {
		e.afterScan()
	}
	file := path.Join(dir, "errors.go")
	src, err := fs.ReadFile(root.FS(), file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := create(root, file, newErrorsFile(path.Base(pkg), id, slug)); err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		row, err := rowType(file, src)
		if err != nil {
			return err
		}
		if err := appendTo(root, file, errorRow(row, id, slug, "")); err != nil {
			return err
		}
	}
	// Two runs at once may take the same number; the table, read again,
	// shows it, and the run that sees it fails.
	after, err := cli.ReadTable(root.FS())
	if err != nil {
		return err
	}
	if n := countID(after, id); n > 1 {
		return fmt.Errorf("another run declared %s at the same time; remove one of the %d rows and run the command again", id, n)
	}
	say(e.stdout, "%s %s %s\n", file, id, slug)
	return nil
}

// countID returns how many declared rows carry id.
func countID(rows []cli.Declared, id string) int {
	n := 0
	for _, r := range rows {
		if r.ID == id {
			n++
		}
	}
	return n
}

// rowType returns how the errors.go at file names the type Row: Row in
// package cli itself, cli.Row in a package that imports it. A file that
// does neither cannot hold a row, so it is an error.
func rowType(file string, src []byte) (string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), file, src, parser.ImportsOnly)
	if err != nil {
		return "", err
	}
	if f.Name.Name == "cli" {
		return "Row", nil
	}
	for _, imp := range f.Imports {
		if p, err := strconv.Unquote(imp.Path.Value); err == nil && p == cliImport && imp.Name == nil {
			return "cli.Row", nil
		}
	}
	return "", fmt.Errorf("%s does not import %s, which declares Row; add the import and run the command again", file, cliImport)
}

// errorRow returns the declaration of one row, its exit code, hint and
// "recovered in" cell to fill. indent starts each line of a row inside
// a var block.
func errorRow(rowType, id, slug, indent string) string {
	name := camel(strings.ReplaceAll(slug, "-", " "))
	lines := []string{
		"// " + name + " is the error " + slug + " (" + id + "). fill: when the package returns it.",
		"var " + name + " = " + rowType + "{",
		"\tID:          " + strconv.Quote(id) + ",",
		"\tSlug:        " + strconv.Quote(slug) + ",",
		"\tExit:        0, // fill: the exit code of 04 4.1",
		"\tFix:         \"fill: the hint, a command or decide: <what>\",",
		"\tRecoveredIn: \"fill: the docs/guide/ anchor of its recovery, or none: <reason>\",",
		"}",
	}
	return indent + strings.Join(lines, "\n"+indent) + "\n"
}

// newErrorsFile returns a new errors.go of package name that declares
// the one row.
func newErrorsFile(name, id, slug string) string {
	return header + "package " + name + "\n\nimport \"" + cliImport + "\"\n\n" + errorRow("cli.Row", id, slug, "")
}

// appendTo adds text at the end of the file at path below root, after
// one empty line.
func appendTo(root *os.Root, file, text string) error {
	f, err := root.OpenFile(file, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	_, err = f.WriteString("\n" + text)
	return errors.Join(err, f.Close())
}
