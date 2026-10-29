// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package callsites

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// source is one parsed Go file of the module.
type source struct {
	rel    string // slash path below the module root
	file   *ast.File
	fset   *token.FileSet
	isTest bool
}

// dir is the directory of the file, below the module root.
func (s source) dir() string { return path.Dir(s.rel) }

// tree is the Go files of a module, by directory.
type tree struct {
	module string
	dirs   map[string][]source
}

// parseModule reads every Go file below root, a module whose path is
// module. It leaves out what the go command leaves out: testdata, and
// directories whose name starts with "." or "_". Build constraints are
// not read: a file for another platform is judged too.
func parseModule(root, module string) (*tree, error) {
	t := &tree{module: module, dirs: map[string][]source{}}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, p, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		s := source{rel: rel, file: file, fset: fset, isTest: strings.HasSuffix(name, "_test.go")}
		t.dirs[s.dir()] = append(t.dirs[s.dir()], s)
		return nil
	})
	return t, err
}

// closure returns the directories of the module that the package at
// entry links: itself and, through the imports of non-test files, every
// package of the module below it.
func (t *tree) closure(entry string) map[string]bool {
	linked := map[string]bool{}
	todo := []string{entry}
	for len(todo) > 0 {
		dir := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if linked[dir] {
			continue
		}
		linked[dir] = true
		for _, s := range t.dirs[dir] {
			if s.isTest {
				continue
			}
			for _, spec := range s.file.Imports {
				imported, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					continue
				}
				if rest, ok := strings.CutPrefix(imported, t.module+"/"); ok && len(t.dirs[rest]) > 0 {
					todo = append(todo, rest)
				}
			}
		}
	}
	return linked
}

// importNames returns the name each import of the file has in it, to
// the path of the import. A version suffix ("/v2") is not a name.
func importNames(file *ast.File) map[string]string {
	names := map[string]string{}
	for _, spec := range file.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(imported)
		if isVersion(name) {
			name = path.Base(path.Dir(imported))
		}
		if spec.Name != nil {
			name = spec.Name.Name
		}
		names[name] = imported
	}
	return names
}

// isVersion reports whether a path element is a major version, "v2".
func isVersion(elem string) bool {
	rest, ok := strings.CutPrefix(elem, "v")
	if !ok || rest == "" {
		return false
	}
	_, err := strconv.Atoi(rest)
	return err == nil
}

// selectorOf returns n as a selector on an imported package, with the
// path of that package.
func selectorOf(n ast.Node, names map[string]string) (*ast.SelectorExpr, string, bool) {
	sel, ok := n.(*ast.SelectorExpr)
	if !ok {
		return nil, "", false
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil, "", false
	}
	imported, ok := names[x.Name]
	return sel, imported, ok
}

// inside reports whether the slash path rel is in one of the dirs; an
// import path is a slash path too.
func inside(rel string, dirs []string) bool {
	return slices.ContainsFunc(dirs, func(dir string) bool { return strings.HasPrefix(rel, dir+"/") })
}
