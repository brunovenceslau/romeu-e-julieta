// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// maxErrorsFile is the size an errors.go may have, far above any
// package's rows, so a stray file cannot make the read unbounded.
const maxErrorsFile = 1 << 20

// Declared is a row of the error table as a package declares it: the
// row, and the slash-separated path of the errors.go that holds it.
type Declared struct {
	Row
	File string
}

// ReadTable returns the union of the error table (04 4.1, Error ids):
// every Row a file named errors.go below internal/ of the tree fsys
// declares as a composite literal, sorted by id. Each field must be a
// literal, so the table can be read without running the code.
func ReadTable(fsys fs.FS) ([]Declared, error) {
	var out []Declared
	err := fs.WalkDir(fsys, "internal", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || path.Base(p) != "errors.go" {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s: an errors.go is a regular file", p)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxErrorsFile {
			return fmt.Errorf("%s: an errors.go holds at most %d bytes", p, maxErrorsFile)
		}
		src, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		rows, err := declaredRows(p, src)
		if err != nil {
			return err
		}
		out = append(out, rows...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(out, func(a, b Declared) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

// declaredRows returns the Row literals of the errors.go at p.
func declaredRows(p string, src []byte) ([]Declared, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, p, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var out []Declared
	var bad error
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !isRowType(lit.Type, file.Name.Name) || bad != nil {
			return true
		}
		row, err := rowOf(lit)
		if err != nil {
			bad = fmt.Errorf("%s: %w", fset.Position(lit.Pos()), err)
			return false
		}
		out = append(out, Declared{Row: row, File: p})
		return false
	})
	return out, bad
}

// isRowType reports whether expr names the type Row: cli.Row, or Row in
// package cli itself.
func isRowType(expr ast.Expr, pkg string) bool {
	switch t := expr.(type) {
	case *ast.SelectorExpr:
		x, ok := t.X.(*ast.Ident)
		return ok && x.Name == "cli" && t.Sel.Name == "Row"
	case *ast.Ident:
		return pkg == "cli" && t.Name == "Row"
	}
	return false
}

// rowOf returns the row a keyed composite literal of literals declares.
func rowOf(lit *ast.CompositeLit) (Row, error) {
	var r Row
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return r, fmt.Errorf("a Row is declared with keyed fields")
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			return r, fmt.Errorf("a Row is declared with keyed fields")
		}
		val, ok := kv.Value.(*ast.BasicLit)
		if !ok {
			return r, fmt.Errorf("the field %s of a Row is a literal", key.Name)
		}
		var err error
		switch key.Name {
		case "ID":
			r.ID, err = strconv.Unquote(val.Value)
		case "Slug":
			r.Slug, err = strconv.Unquote(val.Value)
		case "Fix":
			r.Fix, err = strconv.Unquote(val.Value)
		case "RecoveredIn":
			r.RecoveredIn, err = strconv.Unquote(val.Value)
		case "Exit":
			r.Exit, err = strconv.Atoi(val.Value)
		default:
			err = fmt.Errorf("a Row has no field %s", key.Name)
		}
		if err != nil {
			return r, fmt.Errorf("the field %s of a Row: %w", key.Name, err)
		}
	}
	return r, nil
}

// SpecRow is a row of the list of 04 4.4, which stands until the
// errors are declared in code: its id, slug and exit code, and whether
// it is marked retired.
type SpecRow struct {
	ID      string
	Slug    string
	Exit    int
	Retired bool
}

// specRowLine matches a row of the table of 04 4.4.
var specRowLine = regexp.MustCompile("^\\| `(RJ-[0-9]{3})` \\| `([a-z0-9-]+)` \\| ([0-9]+) \\| (.*)\\|$")

// SpecRows returns the rows of the table under the heading "## 4.4
// Error ids" of the page src (docs/spec/04-cli.md), in order. A row is
// retired when its last cell says "retired".
func SpecRows(src []byte) ([]SpecRow, error) {
	var out []SpecRow
	in := false
	sc := bufio.NewScanner(bytes.NewReader(src))
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "## ") {
			in = strings.HasPrefix(line, "## 4.4 Error ids")
			continue
		}
		if !in {
			continue
		}
		m := specRowLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		exit, err := strconv.Atoi(m[3])
		if err != nil {
			return nil, err
		}
		out = append(out, SpecRow{ID: m[1], Slug: m[2], Exit: exit, Retired: strings.Contains(m[4], "retired")})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no row of the error table under 4.4")
	}
	return out, nil
}

// UnionProblems returns what is wrong with the declared rows against
// the rows of 04 4.4 (04 4.1, Error ids; docs/plan.md rule 5): a
// duplicate id, an id not in 4.4, a declared id that 4.4 marks retired,
// and a slug or exit code that differs from its row. An id of 4.4 that
// no package declares is not a problem here (Pending lists it).
func UnionProblems(declared []Declared, spec []SpecRow) []string {
	var problems []string
	byID := map[string]SpecRow{}
	for _, s := range spec {
		if _, dup := byID[s.ID]; dup {
			problems = append(problems, fmt.Sprintf("04 4.4 lists %s twice", s.ID))
		}
		byID[s.ID] = s
	}
	seen := map[string]string{}
	for _, d := range declared {
		if file, dup := seen[d.ID]; dup {
			problems = append(problems, fmt.Sprintf("%s: %s is declared again, first in %s", d.File, d.ID, file))
			continue
		}
		seen[d.ID] = d.File
		s, ok := byID[d.ID]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s: %s is not a row of 04 4.4", d.File, d.ID))
		case s.Retired:
			problems = append(problems, fmt.Sprintf("%s: %s is retired in 04 4.4", d.File, d.ID))
		case s.Slug != d.Slug || s.Exit != d.Exit:
			problems = append(problems, fmt.Sprintf("%s: %s is %s exit %d, but 04 4.4 says %s exit %d", d.File, d.ID, d.Slug, d.Exit, s.Slug, s.Exit))
		}
	}
	return problems
}

// Pending returns the ids of 04 4.4, retired ones left out, that no
// package declares yet.
func Pending(declared []Declared, spec []SpecRow) []string {
	var out []string
	for _, s := range spec {
		if s.Retired || slices.ContainsFunc(declared, func(d Declared) bool { return d.ID == s.ID }) {
			continue
		}
		out = append(out, s.ID)
	}
	return out
}
