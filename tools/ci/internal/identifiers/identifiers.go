// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package identifiers holds the specification to the code of tools/ci
// (12 12.2): every identifier a page under docs/spec/ names as
// "`<name>` of the <x> package of `tools/ci`" (or "in the") is declared
// at the top level of a package named <x> below tools/ci. The records
// under docs/adr/ are history and are not read. Its test runs it on
// this repository; it registers no check.
package identifiers

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// The pages and the tree this package reads, from the root.
const (
	specDir = "docs/spec"
	toolsCI = "tools/ci"
)

// Ref is one identifier a page names: the page, the name and the
// package name.
type Ref struct {
	Page, Name, Package string
}

// refForm is the form a page names an identifier in, read with every
// run of white space as one space, since a sentence wraps.
var refForm = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_]*)` (?:of|in) the ([a-z][a-z0-9]*) package of `tools/ci`")

// space is a run of white space.
var space = regexp.MustCompile(`\s+`)

// Run reads the pages and the code of the repository at root and returns
// one line per identifier a page names and its package does not declare,
// sorted.
func Run(root string) ([]string, error) {
	refs, err := Refs(filepath.Join(root, specDir))
	if err != nil {
		return nil, err
	}
	decls, err := Declared(filepath.Join(root, toolsCI))
	if err != nil {
		return nil, err
	}
	return Check(refs, decls), nil
}

// Refs returns the identifiers the Markdown pages of dir name, in the
// order of the pages and of their text. Pages below dir are not read.
func Refs(dir string) ([]Ref, error) {
	pages, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("no page in %s", dir)
	}
	var refs []Ref
	for _, p := range pages {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		text := space.ReplaceAllString(string(data), " ")
		for _, m := range refForm.FindAllStringSubmatch(text, -1) {
			refs = append(refs, Ref{Page: filepath.Base(p), Name: m[1], Package: m[2]})
		}
	}
	return refs, nil
}

// Declared returns, per package name, the names declared at the top
// level of the Go files below dir: functions, types, variables and
// constants, methods excluded. Test files, testdata and the directories
// go ignores are not read.
func Declared(dir string) (map[string][]string, error) {
	decls := map[string][]string{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && p != dir && ignored(d.Name()):
			return filepath.SkipDir
		case d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go"):
			return nil
		}
		file, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		pkg := file.Name.Name
		decls[pkg] = append(decls[pkg], topLevel(file)...)
		return nil
	})
	return decls, err
}

// topLevel returns the names a file declares at the top level.
func topLevel(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				names = append(names, d.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					names = append(names, s.Name.Name)
				case *ast.ValueSpec:
					for _, n := range s.Names {
						names = append(names, n.Name)
					}
				}
			}
		}
	}
	return names
}

// ignored reports whether go skips a directory of this name.
func ignored(name string) bool {
	return name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

// Check returns one line for each reference whose package is not below
// tools/ci or does not declare the name, sorted.
func Check(refs []Ref, decls map[string][]string) []string {
	var out []string
	for _, r := range refs {
		names, ok := decls[r.Package]
		var line string
		switch {
		case !ok:
			line = fmt.Sprintf("%s: names %s of the %s package of tools/ci, and tools/ci has no package %s", r.Page, r.Name, r.Package, r.Package)
		case !slices.Contains(names, r.Name):
			line = fmt.Sprintf("%s: names %s of the %s package of tools/ci, which that package does not declare", r.Page, r.Name, r.Package)
		default:
			continue
		}
		if !slices.Contains(out, line) {
			out = append(out, line)
		}
	}
	slices.Sort(out)
	return out
}
