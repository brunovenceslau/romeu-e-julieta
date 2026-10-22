// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schema

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"reflect"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/schemagen"
)

// Page is the reference page of one file format, generated with its
// schema (S10, 12 12.3): the field table, from the format's Go types and
// the doc comment of each field, and the rule table, when the format has
// validation rules. A task that adds a schema adds its page through
// RegisterPage.
type Page struct {
	// Format is the format; the page links its schema.
	Format schemagen.Schema
	// Path is the page's path, docs/reference/<name>.md.
	Path string
	// Title is the page's heading.
	Title string
	// Reader names the page's reader, for its front matter (ADR 0001).
	Reader string
	// Source says what the page is generated from, for its header.
	Source string
	// Intro is the Markdown before the field table.
	Intro string
	// Tags are struct tags the field table shows, a column each; a
	// member without the tag shows its top-level field's, as
	// "<value> (via <field>)".
	Tags []Tag
	// Check, when set, says why the page cannot be written, such as
	// struct tags that disagree with a committed list; the generator
	// then fails.
	Check func() error
	// After is the Markdown after the field table.
	After string
	// Rules are the rows of the rule table; none leaves the table out.
	Rules []RuleRow
}

// Tag is a column of the field table: its header and the struct tag it
// shows.
type Tag struct {
	Header string
	Key    string
}

// RuleRow is a row of the rule table of a page.
type RuleRow struct {
	Field, Rule, Message, Error, Scope, Source string
}

// pages holds the registered pages.
var pages []Page

// RegisterPage adds the page of a format. A second page of one path is a
// programming error, and RegisterPage panics: it runs from an init.
func RegisterPage(p Page) {
	if slices.ContainsFunc(pages, func(o Page) bool { return o.Path == p.Path }) {
		panic(fmt.Sprintf("schema: the page %s is registered twice", p.Path))
	}
	pages = append(pages, p)
}

// PageOutputs returns the registered pages, written from the tree at
// root: the files "go generate ./..." writes and the generated check
// holds the tree to.
func PageOutputs(root *os.Root) ([]check.Output, error) {
	return pageOutputs(root, pages)
}

// pageOutputs returns the outputs of list. A page whose format's source
// is not in the tree, such as in a fixture of the generated check, has
// none.
func pageOutputs(root *os.Root, list []Page) ([]check.Output, error) {
	var outs []check.Output
	for _, p := range list {
		dir, err := sourceDir(root.FS(), p.Format.Type)
		if err != nil {
			return nil, err
		}
		if _, err := fs.Stat(root.FS(), dir); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		content, err := PageOf(root, p)
		if err != nil {
			return nil, err
		}
		outs = append(outs, check.Output{Path: p.Path, Content: content})
	}
	return outs, nil
}

// PageOf returns the page p, reading the doc comments of its format's
// types from their source in the tree at root.
func PageOf(root *os.Root, p Page) ([]byte, error) {
	if p.Check != nil {
		if err := p.Check(); err != nil {
			return nil, fmt.Errorf("%s: %w", p.Path, err)
		}
	}
	fields, err := schemagen.Fields(p.Format.Type)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p.Path, err)
	}
	docs := map[reflect.Type]map[string]string{}
	for _, f := range fields {
		if _, ok := docs[f.Owner]; !ok {
			if docs[f.Owner], err = fieldDocs(root.FS(), f.Owner); err != nil {
				return nil, fmt.Errorf("%s: %w", p.Path, err)
			}
		}
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "---\ntype: reference\nreader: %s\n---\n\n", p.Reader)
	b.WriteString(check.HTMLComment(check.GeneratedHeader(p.Source)) + "\n")
	fmt.Fprintf(&b, "# %s\n\n%s\n\n", p.Title, p.Intro)
	fmt.Fprintf(&b, "Its JSON Schema is [%s](../../%s).\n\n", p.Format.Path(), p.Format.Path())
	b.WriteString("## Fields\n\n| Field | Type | Required |")
	for _, t := range p.Tags {
		b.WriteString(" " + t.Header + " |")
	}
	b.WriteString(" Meaning |\n|---|---|---|" + strings.Repeat("---|", len(p.Tags)) + "---|\n")
	for _, f := range fields {
		required := "no"
		if f.Required {
			required = "yes"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s |", cell(f.Path), f.Type, required)
		for _, t := range p.Tags {
			b.WriteString(" " + orDash(cell(tagOf(fields, f, t.Key))) + " |")
		}
		b.WriteString(" " + orDash(cell(docs[f.Owner][f.Field.Name])) + " |\n")
	}
	if p.After != "" {
		b.WriteString("\n" + p.After + "\n")
	}
	if len(p.Rules) > 0 {
		b.WriteString("\n## Rules\n\n| Field | Rule | Message | Error | Checked by | Source |\n|---|---|---|---|---|---|\n")
		for _, r := range p.Rules {
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s | %s |\n", cell(r.Field), cell(r.Rule), cell(r.Message), cell(r.Error), r.Scope, r.Source)
		}
	}
	return b.Bytes(), nil
}

// tagOf returns the struct tag key of the field f, or, for a member
// without one, its top-level field's, "<value> (via <field>)".
func tagOf(fields []schemagen.Field, f schemagen.Field, key string) string {
	if v := f.Field.Tag.Get(key); v != "" {
		return v
	}
	top := f.Path[:strings.IndexAny(f.Path+".", ".[{")]
	if top == f.Path {
		return ""
	}
	for _, o := range fields {
		if o.Path == top {
			if v := o.Field.Tag.Get(key); v != "" {
				return v + " (via " + top + ")"
			}
		}
	}
	return ""
}

// cell returns s for a cell of a Markdown table: each "|" escaped, and
// each "<" outside a code span, so that a placeholder such as <name> is
// not read as an HTML tag.
func cell(s string) string {
	parts := strings.Split(s, "`")
	for i := 0; i < len(parts); i += 2 {
		parts[i] = strings.ReplaceAll(parts[i], "<", `\<`)
	}
	return strings.ReplaceAll(strings.Join(parts, "`"), "|", `\|`)
}

// orDash returns s, or "-" for an empty cell.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// sourceDir returns the slash-separated directory of the package of t
// in the tree fsys, from the module path its go.mod names.
func sourceDir(fsys fs.FS, t reflect.Type) (string, error) {
	mod, err := fs.ReadFile(fsys, goModFile)
	if err != nil {
		return "", err
	}
	m := modulePath.FindSubmatch(mod)
	if m == nil {
		return "", errors.New("go.mod names no module")
	}
	dir, ok := strings.CutPrefix(t.PkgPath(), string(m[1])+"/")
	if !ok {
		return "", fmt.Errorf("the type %s is not of the module %s", t, m[1])
	}
	return dir, nil
}

// fieldDocs returns the doc comment of each field of the struct type t,
// by field name, as one line, read from the non-test Go files of its
// package's directory in the tree fsys.
func fieldDocs(fsys fs.FS, t reflect.Type) (map[string]string, error) {
	dir, err := sourceDir(fsys, t)
	if err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || path.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := fs.ReadFile(fsys, dir+"/"+name)
		if err != nil {
			return nil, err
		}
		file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			spec, ok := n.(*ast.TypeSpec)
			if !ok || spec.Name.Name != t.Name() {
				return true
			}
			st, ok := spec.Type.(*ast.StructType)
			if !ok {
				return false
			}
			for _, f := range st.Fields.List {
				for _, id := range f.Names {
					out[id.Name] = strings.Join(strings.Fields(f.Doc.Text()), " ")
				}
			}
			return false
		})
	}
	return out, nil
}
