// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package moduletree checks that three lists agree on the package
// directories under internal/ (12 12.2): the tree of 02 2.1, the module
// table of 12 12.2 and the directories S9 measures (coverTrees of the
// coverage package). It also checks the packages on disk against them,
// and refuses a catch-all package name (10 10.6). Its test runs it on
// this repository; it registers no check.
package moduletree

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The pages and the tree this package reads, from the root.
const (
	treePage   = "docs/spec/02-layouts.md"
	modulePage = "docs/spec/12-engineering.md"
	toolsCI    = "tools/ci"
	internal   = "internal"
)

// catchAll are the package names 10 10.6 refuses: a package has one
// purpose, and these name none.
var catchAll = []string{"util", "utils", "helper", "helpers", "common"}

// Module is one row of the module table of 12 12.2: its id, and the
// paths under internal/ that its row gives in code spans.
type Module struct {
	ID    string
	Paths []string
}

// Run reads the lists of the repository at root and returns one line
// per directory they disagree on, sorted.
func Run(root string) ([]string, error) {
	page, err := os.ReadFile(filepath.Join(root, treePage))
	if err != nil {
		return nil, err
	}
	tree, err := TreeDirs(page)
	if err != nil {
		return nil, err
	}
	page, err = os.ReadFile(filepath.Join(root, modulePage))
	if err != nil {
		return nil, err
	}
	modules, err := Modules(page)
	if err != nil {
		return nil, err
	}
	cover, err := CoverTrees(filepath.Join(root, toolsCI))
	if err != nil {
		return nil, err
	}
	disk, err := PackageDirs(root)
	if err != nil {
		return nil, err
	}
	return Check(tree, modules, cover, disk), nil
}

// treeEntry is one line of the tree: the indent before the branch, and
// the first word after it.
var treeEntry = regexp.MustCompile(`^((?:│  |   )*)[├└]─ (\S+)`)

// TreeDirs returns the directories the first fenced block of section 2.1
// names, as slash paths from the root, in the order of the page.
func TreeDirs(page []byte) ([]string, error) {
	_, after, ok := bytes.Cut(page, []byte("\n## 2.1 "))
	if !ok {
		return nil, errors.New("no section 2.1 in the tree page")
	}
	_, block, ok := bytes.Cut(after, []byte("\n```"))
	if !ok {
		return nil, errors.New("no fenced block in section 2.1")
	}
	block, _, ok = bytes.Cut(block, []byte("\n```"))
	if !ok {
		return nil, errors.New("the fenced block of section 2.1 is not closed")
	}
	// parents[i] is the directory the lines at depth i+1 sit in. A line
	// such as "memstore/write/" names two levels, but its children are
	// indented one level below it.
	var dirs, parents []string
	sc := bufio.NewScanner(bytes.NewReader(block))
	for sc.Scan() {
		m := treeEntry.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		depth := utf8.RuneCountInString(m[1]) / 3 // "│  " and "   " are three runes each
		if depth > len(parents) {
			return nil, fmt.Errorf("the tree line %q is indented below no directory", sc.Text())
		}
		parents = parents[:depth]
		name, isDir := strings.CutSuffix(m[2], "/")
		if !isDir {
			continue
		}
		dir := name
		if depth > 0 {
			dir = parents[depth-1] + "/" + name
		}
		dirs = append(dirs, dir)
		parents = append(parents, dir)
	}
	return dirs, sc.Err()
}

// moduleRow is a row of the module table, and internalSpan a path under
// internal/ in a code span of its cells.
var (
	moduleRow    = regexp.MustCompile("^\\| `([a-z0-9-]+)` \\|(.*)$")
	internalSpan = regexp.MustCompile("`(internal/[^`\\s]*)`")
)

// Modules returns the rows of the table of 12 12.2 whose header starts
// with "| Module id |".
func Modules(page []byte) ([]Module, error) {
	_, table, ok := bytes.Cut(page, []byte("\n| Module id |"))
	if !ok {
		return nil, errors.New("no module table in the module page")
	}
	var modules []Module
	lines := strings.Split(string(table), "\n")
	for _, line := range lines[1:] {
		if !strings.HasPrefix(line, "|") {
			break
		}
		m := moduleRow.FindStringSubmatch(line)
		if m == nil {
			continue // the separator row
		}
		mod := Module{ID: m[1]}
		for _, s := range internalSpan.FindAllStringSubmatch(m[2], -1) {
			mod.Paths = append(mod.Paths, strings.TrimSuffix(s[1], "/"))
		}
		modules = append(modules, mod)
	}
	if len(modules) == 0 {
		return nil, errors.New("the module table has no row")
	}
	return modules, nil
}

// CoverTrees returns the string literals of the one package-level var
// named coverTrees in the Go files below dir, test files excluded.
func CoverTrees(dir string) ([]string, error) {
	var found [][]string
	err := goFiles(dir, func(file *ast.File) error {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, name := range vs.Names {
					if name.Name != "coverTrees" || i >= len(vs.Values) {
						continue
					}
					trees, err := stringList(vs.Values[i])
					if err != nil {
						return err
					}
					found = append(found, trees)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(found) != 1 {
		return nil, fmt.Errorf("%d declarations of coverTrees below %s, want 1", len(found), dir)
	}
	return found[0], nil
}

// stringList returns the strings of a []string literal.
func stringList(e ast.Expr) ([]string, error) {
	lit, ok := e.(*ast.CompositeLit)
	if !ok {
		return nil, errors.New("coverTrees is not a literal")
	}
	var out []string
	for _, elt := range lit.Elts {
		b, ok := elt.(*ast.BasicLit)
		if !ok || b.Kind != token.STRING {
			return nil, errors.New("coverTrees holds a value that is not a string literal")
		}
		s, err := strconv.Unquote(b.Value)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// goFiles parses each Go file below dir that is not a test file, and
// skips testdata and the directories go ignores.
func goFiles(dir string, each func(*ast.File) error) error {
	fset := token.NewFileSet()
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != dir && ignored(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		return each(file)
	})
}

// ignored reports whether go skips a directory of this name.
func ignored(name string) bool {
	return name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

// PackageDirs returns the directories below internal/ at root that hold
// a Go file, as slash paths from the root, sorted. No internal/ is no
// directory.
func PackageDirs(root string) ([]string, error) {
	var dirs []string
	top := filepath.Join(root, internal)
	err := filepath.WalkDir(top, func(p string, d fs.DirEntry, err error) error {
		switch {
		case errors.Is(err, fs.ErrNotExist) && p == top:
			return filepath.SkipAll
		case err != nil:
			return err
		case d.IsDir() && p != top && ignored(d.Name()):
			return filepath.SkipDir
		case d.IsDir() || !strings.HasSuffix(p, ".go"):
			return nil
		}
		rel, err := filepath.Rel(root, filepath.Dir(p))
		if err != nil {
			return err
		}
		if rel = filepath.ToSlash(rel); !slices.Contains(dirs, rel) {
			dirs = append(dirs, rel)
		}
		return nil
	})
	slices.Sort(dirs)
	return dirs, err
}

// Check returns one line for each disagreement, sorted:
//   - a directory under internal/ in the tree that no module row owns;
//   - a module row whose id names no directory of the tree, when the id
//     is a directory name (no hyphen), and a path under internal/ that
//     a row gives and the tree lacks;
//   - a directory of the tree or the disk under internal/ that is below
//     none of coverTrees;
//   - a package on disk under internal/ that no directory of the tree
//     holds or names, or that no module row owns;
//   - a directory under internal/ with a catch-all name.
//
// A tree segment written <name> stands for any one directory.
func Check(tree []string, modules []Module, coverTrees, disk []string) []string {
	var out []string
	add := func(format string, args ...any) {
		if line := fmt.Sprintf(format, args...); !slices.Contains(out, line) {
			out = append(out, line)
		}
	}
	var inTree []string
	for _, d := range tree {
		if below(d, internal) && d != internal {
			inTree = append(inTree, d)
		}
	}
	for _, d := range inTree {
		if !owned(d, modules) {
			add("%s: in the tree of 02 2.1, and no row of the module table of 12 12.2 owns it", d)
		}
	}
	// An id names a directory when some directory of the tree, anywhere,
	// has that name: "probes" is e2e/probes. That is all REQ-1082 asks,
	// so a row is not held to internal/<id>.
	for _, m := range modules {
		if !strings.Contains(m.ID, "-") && !slices.ContainsFunc(tree, func(d string) bool { return path.Base(d) == m.ID }) {
			add("module %s: names no directory of the tree of 02 2.1", m.ID)
		}
		for _, p := range m.Paths {
			if !slices.ContainsFunc(inTree, func(d string) bool { return prefix(p, d) }) {
				add("module %s: gives %s, which the tree of 02 2.1 lacks", m.ID, p)
			}
		}
	}
	for _, d := range slices.Concat(inTree, disk) {
		if !slices.ContainsFunc(coverTrees, func(c string) bool { return below(d, c) }) {
			add("%s: below none of coverTrees, so S9 does not measure it", d)
		}
		for seg := range strings.SplitSeq(d, "/") {
			if slices.Contains(catchAll, seg) {
				add("%s: %q names no purpose; a package does one thing (10 10.6)", d, seg)
			}
		}
	}
	for _, d := range disk {
		if !slices.ContainsFunc(inTree, func(t string) bool { return prefix(d, t) || prefix(t, d) }) {
			add("%s: a package the tree of 02 2.1 lacks", d)
		}
		if !owned(d, modules) {
			add("%s: a package no row of the module table of 12 12.2 owns", d)
		}
	}
	slices.Sort(out)
	return out
}

// owned reports whether a module owns dir: internal/<id> or a path its
// row gives, or a directory below one of them.
func owned(dir string, modules []Module) bool {
	for _, m := range modules {
		if below(dir, path.Join(internal, m.ID)) {
			return true
		}
		for _, p := range m.Paths {
			if prefix(p, dir) {
				return true
			}
		}
	}
	return false
}

// below reports whether dir is top or a directory below it.
func below(dir, top string) bool {
	return dir == top || strings.HasPrefix(dir, top+"/")
}

// prefix reports whether the segments of p start the segments of dir,
// where a segment written <name> in either matches any one segment.
func prefix(p, dir string) bool {
	ps, ds := strings.Split(p, "/"), strings.Split(dir, "/")
	if len(ps) > len(ds) {
		return false
	}
	for i, s := range ps {
		if s != ds[i] && !placeholder(s) && !placeholder(ds[i]) {
			return false
		}
	}
	return true
}

// placeholder reports whether a segment is written <name>.
func placeholder(s string) bool {
	return len(s) > 2 && s[0] == '<' && s[len(s)-1] == '>'
}
