// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package catalog is the egress catalog (03 3.5, REQ-0040): which
// domains each mise backend:tool needs, the fixed meta and base sets,
// and an upload flag per domain. The entries are the files
// egress/*.yaml, one per tool family, embedded in romeu and julieta by
// glob (REQ-0128, REQ-0129, REQ-0638); the meta and base sets live in
// egress/base.yaml.
//
// A catalog file is a file the maintainer writes, so its reader refuses
// an unknown key, a typing error most often (REQ-0163, ADR 0041: the
// catalog is in the group of files a person writes, whose readers stay
// strict; a program never writes it, so no newer writer can add a member
// an older reader must skip). Every domain, from any source of the
// file, passes the one domain rule of internal/spec (REQ-0188). The
// reader also refuses what hides a value from a reviewer: an anchor, an
// alias, a merge key, a null, a tag outside YAML's core schema, and an
// upload flag that is not written true or false (yes and on are
// refused). Whether a domain is gated is Gated: a missing entry is
// gated, so no caller reads the flag of a domain the catalog lacks.
//
// Decode reads one file and Build merges the files: a domain, or a tool,
// that two files declare with different entries is a decode error. That
// every domain a file uses has an upload entry is not a decode rule
// but the check "tools/ci catalog" (REQ-0212, REQ-0641), through
// Catalog.Undeclared; at run time a domain without an entry is gated
// (07 7.5, step 7).
package catalog

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// Version is the n of the schema field "catalog/v<n>" this build reads.
const Version = 1

// Field is the schema field of a catalog file (REQ-0161).
const Field = "catalog/v1"

// BaseFile is the file that holds the meta and base sets (03 3.5).
const BaseFile = "base.yaml"

// MaxFileSize is the most bytes of a catalog file, the file cap of
// 07 7.5, step 1.
const MaxFileSize = 1 << 20

// ErrInvalid is wrapped by every refusal of Decode and Build.
var ErrInvalid = errors.New("catalog: not a valid catalog")

// File is one catalog file, the document of the format catalog.v1.
type File struct {
	// Schema is "catalog/v1".
	Schema string `json:"schema" yaml:"schema"`
	// Meta are the domains of mise itself, needed by every project; only
	// base.yaml holds them.
	Meta []string `json:"meta,omitzero" yaml:"meta,omitempty"`
	// Base are the domains of git over HTTPS, needed by every project;
	// only base.yaml holds them.
	Base []string `json:"base,omitzero" yaml:"base,omitempty"`
	// Domains holds the upload flag of each domain; CI refuses a domain
	// used anywhere without an entry here.
	Domains map[string]Domain `json:"domains,omitzero" yaml:"domains,omitempty"`
	// Tools maps a mise backend:tool, as written in mise.lock, to the
	// domains it needs to install and to run.
	Tools map[string][]string `json:"tools,omitzero" yaml:"tools,omitempty"`
}

// Domain is the entry of a domain.
type Domain struct {
	// Upload says that data can be sent out to the domain, as to a
	// multi-tenant object store; such a domain is always gated.
	Upload *bool `json:"upload" yaml:"upload"`
}

// toolKey is the form of a tool key: the backend of mise, a colon, and
// the tool as the backend names it, such as aqua:jqlang/jq or
// npm:@angular/cli.
const toolKey = `^[a-z][a-z0-9_-]*:[A-Za-z0-9@][A-Za-z0-9._/@+~-]*$`

// ToolKeyPattern is the pattern of a tool key, for the generated schema.
const ToolKeyPattern = toolKey

var toolKeyForm = regexp.MustCompile(toolKey)

// ValidTool returns why key is not a tool key, or nil.
func ValidTool(key string) error {
	if !toolKeyForm.MatchString(key) {
		return fmt.Errorf("the key %q is not a backend:tool of mise.lock", clip(key))
	}
	return nil
}

// Decode reads one catalog file strictly: at most MaxFileSize bytes of
// UTF-8, one YAML mapping, one document, no key that the format lacks
// and none written twice, the schema field, and every value valid.
func Decode(data []byte) (File, error) {
	if len(data) > MaxFileSize {
		return File{}, fmt.Errorf("%w: the file has %d bytes, over the cap of %d", ErrInvalid, len(data), MaxFileSize)
	}
	if !utf8.Valid(data) {
		return File{}, fmt.Errorf("%w: the file is not UTF-8", ErrInvalid)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return File{}, fmt.Errorf("%w: %s", ErrInvalid, yamlError(err))
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return File{}, fmt.Errorf("%w: the file is not one mapping", ErrInvalid)
	}
	if err := checkTree(doc.Content[0]); err != nil {
		return File{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := checkUpload(doc.Content[0]); err != nil {
		return File{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	var f File
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return File{}, fmt.Errorf("%w: %s", ErrInvalid, yamlError(err))
	}
	if err := dec.Decode(new(yaml.Node)); !errors.Is(err, io.EOF) {
		return File{}, fmt.Errorf("%w: the file holds more than one document", ErrInvalid)
	}
	if err := f.validate(); err != nil {
		return File{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return f, nil
}

// coreTags are the tags of YAML's core schema a value may resolve to.
var coreTags = []string{"!!str", "!!int", "!!bool", "!!float", "!!seq", "!!map"}

// checkTree returns why the node n, or one below it, is outside the
// subset a reviewer can read at a glance: no anchor, alias or merge key,
// no null (leave the key out) and no tag outside YAML's core schema.
func checkTree(n *yaml.Node) error {
	switch {
	case n.Kind == yaml.AliasNode:
		return fmt.Errorf("line %d: an alias", n.Line)
	case n.Anchor != "":
		return fmt.Errorf("line %d: an anchor", n.Line)
	case n.Tag == "!!null":
		return fmt.Errorf("line %d: a null value; leave the key out instead", n.Line)
	case n.Tag == "!!merge":
		return fmt.Errorf("line %d: a merge key", n.Line)
	case !slices.Contains(coreTags, n.Tag):
		return fmt.Errorf("line %d: a tag outside YAML's core schema", n.Line)
	}
	for _, c := range n.Content {
		if err := checkTree(c); err != nil {
			return err
		}
	}
	return nil
}

// checkUpload returns why an upload flag of the file's domains is not the
// plain true or false (a quoted one included): go.yaml.in/yaml/v3 reads yes, on and the like as
// booleans too (YAML 1.1), and a reviewer reads those as words.
func checkUpload(root *yaml.Node) error {
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "domains" {
			continue
		}
		for _, entry := range root.Content[i+1].Content {
			for j := 0; j+1 < len(entry.Content); j += 2 {
				if v := entry.Content[j+1]; entry.Content[j].Value == "upload" &&
					(v.Kind != yaml.ScalarNode || v.Tag != "!!bool" || v.Value != "true" && v.Value != "false") {
					return fmt.Errorf("line %d: the upload flag is written true or false", v.Line)
				}
			}
		}
	}
	return nil
}

// yamlLine finds the line number in an error of go.yaml.in/yaml/v3.
var yamlLine = regexp.MustCompile(`line ([0-9]+)`)

// yamlError says what an error of go.yaml.in/yaml/v3 means, with its
// line, without its text, which quotes the keys and values.
func yamlError(err error) string {
	msg := err.Error()
	what := "the file is not YAML the format reads"
	switch {
	case strings.Contains(msg, "not found in type"):
		what = "a key the format does not have"
	case strings.Contains(msg, "already defined"), strings.Contains(msg, "already set"):
		what = "a key written twice"
	case strings.Contains(msg, "cannot unmarshal"):
		what = "a value of the wrong kind"
	}
	if m := yamlLine.FindStringSubmatch(msg); m != nil {
		return "line " + m[1] + ": " + what
	}
	return what
}

// validate judges the values of a decoded file, in the order of the file's
// own layout, so a file with two faults reports the same one every time.
func (f File) validate() error {
	if f.Schema != Field {
		return fmt.Errorf("the schema field is %q, this build reads %q", clip(f.Schema), Field)
	}
	for _, set := range []struct {
		name    string
		domains []string
	}{{"meta", f.Meta}, {"base", f.Base}} {
		for i, d := range set.domains {
			if err := spec.ValidDomain(d); err != nil {
				return fmt.Errorf("%s[%d]: %w", set.name, i, err)
			}
		}
	}
	for _, d := range slices.Sorted(maps.Keys(f.Domains)) {
		if err := spec.ValidDomain(d); err != nil {
			return fmt.Errorf("domains: %w", err)
		}
		if f.Domains[d].Upload == nil {
			return fmt.Errorf("domains.%s has no upload flag, true or false", d)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(f.Tools)) {
		if err := ValidTool(k); err != nil {
			return fmt.Errorf("tools: %w", err)
		}
		if f.Tools[k] == nil {
			return fmt.Errorf("tools.%s is null; write [] for a tool that needs no domain", k)
		}
		for i, d := range f.Tools[k] {
			if err := spec.ValidDomain(d); err != nil {
				return fmt.Errorf("tools.%s[%d]: %w", k, i, err)
			}
		}
	}
	return nil
}

// Source is a catalog file by name, a base name such as "go.yaml".
type Source struct {
	Name string
	Data []byte
}

// Catalog is the merge of the files: the lookups of 03 3.5.
type Catalog struct {
	meta, base []string
	upload     map[string]bool
	tools      map[string][]string
	used       map[string]bool
}

// Build decodes and merges the files. The meta and base sets are
// base.yaml's alone, which must be there. A domain, or a tool, that two
// files declare with different entries is an error naming both files; the
// same entry twice is not. Every fault is reported, one per line, and each
// names its file.
func Build(sources []Source) (Catalog, error) {
	sources = slices.SortedFunc(slices.Values(sources), func(a, b Source) int { return strings.Compare(a.Name, b.Name) })
	c := Catalog{upload: map[string]bool{}, tools: map[string][]string{}, used: map[string]bool{}}
	var errs []error
	fail := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf("%w: "+format, append([]any{ErrInvalid}, args...)...))
	}
	declaredIn := map[string]string{} // a domain or a tool to the file that declared it first
	haveBase := false
	for _, s := range sources {
		f, err := Decode(s.Data)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Name, err))
			continue
		}
		if s.Name == BaseFile {
			haveBase = true
			c.meta, c.base = sortedUnique(f.Meta), sortedUnique(f.Base)
		} else if len(f.Meta) > 0 || len(f.Base) > 0 {
			fail("%s: meta and base belong in %s", s.Name, BaseFile)
		}
		for _, d := range slices.Sorted(maps.Keys(f.Domains)) {
			up := *f.Domains[d].Upload
			if old, ok := c.upload[d]; ok && old != up {
				fail("the domain %s is declared with different entries in %s and %s", d, declaredIn["d:"+d], s.Name)
				continue
			}
			c.upload[d] = up
			if _, ok := declaredIn["d:"+d]; !ok {
				declaredIn["d:"+d] = s.Name
			}
		}
		for _, k := range slices.Sorted(maps.Keys(f.Tools)) {
			domains := sortedUnique(f.Tools[k])
			if old, ok := c.tools[k]; ok && !slices.Equal(old, domains) {
				fail("the tool %s is declared with different entries in %s and %s", k, declaredIn["t:"+k], s.Name)
				continue
			}
			c.tools[k] = domains
			if _, ok := declaredIn["t:"+k]; !ok {
				declaredIn["t:"+k] = s.Name
			}
		}
	}
	if !haveBase && len(errs) == 0 {
		fail("there is no %s, which holds the meta and base sets", BaseFile)
	}
	if len(errs) > 0 {
		return Catalog{}, errors.Join(errs...)
	}
	for _, d := range c.meta {
		c.used[d] = true
	}
	for _, d := range c.base {
		c.used[d] = true
	}
	for _, ds := range c.tools {
		for _, d := range ds {
			c.used[d] = true
		}
	}
	return c, nil
}

// sortedUnique returns the sorted set of list.
func sortedUnique(list []string) []string {
	return slices.Compact(slices.Sorted(slices.Values(list)))
}

// Meta returns the domains of mise itself, needed by every project.
func (c Catalog) Meta() []string { return slices.Clone(c.meta) }

// Base returns the domains of git over HTTPS, needed by every project.
func (c Catalog) Base() []string { return slices.Clone(c.base) }

// Tool returns the domains a backend:tool needs, and whether the catalog
// has the tool: a tool that needs none is found with an empty list.
func (c Catalog) Tool(key string) ([]string, bool) {
	d, ok := c.tools[key]
	return slices.Clone(d), ok
}

// Tools returns the tool keys, sorted.
func (c Catalog) Tools() []string { return slices.Sorted(maps.Keys(c.tools)) }

// Gated reports whether a domain is gated: its upload flag is true, or
// the catalog has no entry for it (07 7.5, step 7). The flag alone is not
// public, so no caller can read a missing entry as "no upload".
func (c Catalog) Gated(domain string) bool {
	upload, declared := c.upload[domain]
	return upload || !declared
}

// Undeclared returns the domains used by the meta and base sets or by a
// tool that have no entry in any file's domains, sorted: what the check
// "tools/ci catalog" refuses (REQ-0212, REQ-0641).
func (c Catalog) Undeclared() []string {
	var out []string
	for d := range c.used {
		if _, ok := c.upload[d]; !ok {
			out = append(out, d)
		}
	}
	slices.Sort(out)
	return out
}

//go:embed egress/*.yaml
var files embed.FS

// Embedded returns the catalog the binaries embed, built from
// egress/*.yaml (REQ-0129). A fault is a defect of the build, which CI
// refuses (tools/ci catalog), so it is returned and not repaired.
var Embedded = sync.OnceValues(func() (Catalog, error) {
	entries, err := files.ReadDir("egress")
	if err != nil {
		return Catalog{}, err
	}
	var sources []Source
	for _, e := range entries {
		data, err := files.ReadFile(path.Join("egress", e.Name()))
		if err != nil {
			return Catalog{}, err
		}
		sources = append(sources, Source{Name: e.Name(), Data: data})
	}
	return Build(sources)
})

// clipLen is the most bytes of an input a message quotes.
const clipLen = 64

// clip returns s cut to clipLen bytes at a character boundary.
func clip(s string) string {
	if len(s) <= clipLen {
		return s
	}
	cut := clipLen
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "..."
}
