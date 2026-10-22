// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package spec holds the formats a person writes for romeu (03): the
// project spec (project.v1, 03 3.2) with its run layout, and the host
// settings (host-settings.v1, 03 3.4). It has their Go types, with the
// gate and apply tags that classify each field of a project (01 1.4),
// their strict decoders, the reader of their schema field
// (versions.go), and the name and path rules of 03 3.1 as data
// (rules.go), from which the validators, the JSON Schemas and the rule
// tables of the reference pages are derived (REQ-0172).
//
// Both formats are files a person writes, so their readers refuse an
// unknown key, which there is most often a typing error (REQ-0163, ADR
// 0041).
package spec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// MaxFileSize is the file cap of 07 7.5, step 1: a spec or host
// settings file larger than 1 MiB is refused before it is parsed
// (REQ-0168).
const MaxFileSize = 1 << 20

// maxDepth bounds the nesting of a decoded file; the run layout's trees
// are the deepest part of a spec.
const maxDepth = 64

// ErrInvalid is wrapped by every refusal of a decoder or a validator but
// those of versions.go: a file that is not a valid document of its
// format.
var ErrInvalid = errors.New("spec: not a valid document")

// DecodeProject reads a project spec strictly and validates it by the
// rules of rules.go that read the spec alone (Validate); the rules that
// read other specs or host settings are ValidateSet's.
func DecodeProject(data []byte) (Project, error) {
	var p Project
	if err := decode(data, project, &p); err != nil {
		return Project{}, err
	}
	p.setDefaults()
	if err := p.Validate(); err != nil {
		return Project{}, err
	}
	return p, nil
}

// DecodeHostSettings reads host settings strictly and validates them by
// the rules of rules.go (Validate).
func DecodeHostSettings(data []byte) (HostSettings, error) {
	var h HostSettings
	if err := decode(data, hostSettings, &h); err != nil {
		return HostSettings{}, err
	}
	if err := h.Validate(); err != nil {
		return HostSettings{}, err
	}
	return h, nil
}

// decode reads data, a file of the format f, into out, a pointer to the
// format's type. In order: at most MaxFileSize bytes of UTF-8; one YAML
// mapping in the strict subset grammar (no anchor, alias, merge key or
// tag outside YAML's core schema, which also refuses an include, no
// duplicate key, no null, nested at most maxDepth levels, REQ-0168);
// the schema field (versions.go), read from the parsed mapping before
// the grammar is judged, so a newer file is format-newer whatever it
// holds; every key the type requires present and each value of the kind
// its field has, as the generated schema says; then go.yaml.in/yaml/v3
// with KnownFields(true), so an unknown key is an error too (REQ-0960).
// The text of an error names a line and a path, never a key or a value
// of the file, which can be a secret or an escape sequence.
//
//romeu:invariant I12 guard
func decode(data []byte, f format, out any) error {
	if len(data) > MaxFileSize {
		return fmt.Errorf("%w: the file has %d bytes, over the cap of %d", ErrInvalid, len(data), MaxFileSize)
	}
	if !utf8.Valid(data) {
		return fmt.Errorf("%w: the file is not UTF-8", ErrInvalid)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		if newer := f.newer(data); newer != nil {
			return newer
		}
		return fmt.Errorf("%w: %s", ErrInvalid, yamlError(err))
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("%w: the file is not one mapping", ErrInvalid)
	}
	root := doc.Content[0]
	if err := f.checkVersion(root); err != nil {
		return err
	}
	nodes := 0
	if err := checkTree(root, 0, &nodes); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := checkShape(root, reflect.TypeOf(out).Elem(), ""); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalid, yamlError(err))
	}
	if err := dec.Decode(new(yaml.Node)); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: the file holds more than one document", ErrInvalid)
	}
	return nil
}

// yamlLine finds the line number in an error of go.yaml.in/yaml/v3.
var yamlLine = regexp.MustCompile(`line ([0-9]+)`)

// yamlError says what an error of go.yaml.in/yaml/v3 means, with its
// line, without its text, which quotes the keys and the values of the
// file.
func yamlError(err error) string {
	msg := err.Error()
	what := "the file is not YAML the format reads"
	switch {
	case strings.Contains(msg, "not found in type"):
		what = "a key the format does not have"
	case strings.Contains(msg, "already defined"):
		what = "a key written twice"
	case strings.Contains(msg, "cannot unmarshal"):
		what = "a value of the wrong kind"
	}
	if m := yamlLine.FindStringSubmatch(msg); m != nil {
		return "line " + m[1] + ": " + what
	}
	return what
}

// maxNodes bounds the nodes of a decoded file, so a file under the size
// cap cannot make the validators walk millions of values.
const maxNodes = 65536

// coreTags are the tags of YAML's core schema a value may resolve to.
var coreTags = []string{"!!str", "!!int", "!!bool", "!!float", "!!seq", "!!map"}

// checkTree returns why the node n, or one below it, is outside the
// subset grammar; nodes counts the nodes it has seen.
func checkTree(n *yaml.Node, depth int, nodes *int) error {
	if *nodes++; *nodes > maxNodes {
		return fmt.Errorf("the file holds more than %d nodes", maxNodes)
	}
	switch {
	case depth > maxDepth:
		return fmt.Errorf("the file nests deeper than %d levels", maxDepth)
	case n.Kind == yaml.AliasNode:
		return fmt.Errorf("line %d: an alias", n.Line)
	case n.Anchor != "":
		return fmt.Errorf("line %d: an anchor", n.Line)
	case n.Tag == "!!null":
		return fmt.Errorf("line %d: a null value; leave the key out instead", n.Line)
	case n.Tag == "!!merge" && n.Kind == yaml.ScalarNode:
		// A merge key is refused with the keys below, which must be
		// strings; a value "<<" is the string, which go.yaml.in/yaml/v3
		// writes plain and reads back with this tag.
		n.Tag = "!!str"
	case !slices.Contains(coreTags, n.Tag):
		return fmt.Errorf("line %d: a tag outside YAML's core schema", n.Line)
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]
			switch {
			case k.Tag == "!!merge":
				return fmt.Errorf("line %d: a merge key", k.Line)
			case k.Kind != yaml.ScalarNode || k.Tag != "!!str":
				return fmt.Errorf("line %d: a key that is not a string", k.Line)
			}
			if seen[k.Value] {
				return fmt.Errorf("line %d: a key is written twice", k.Line)
			}
			seen[k.Value] = true
		}
	}
	for _, c := range n.Content {
		if err := checkTree(c, depth+1, nodes); err != nil {
			return err
		}
	}
	return nil
}

// checkShape returns why the node n, a value of the Go type t at the
// path at, is not of the kind t is, or lacks a key t requires: a field
// without omitzero is required, as in the generated schema. A key t
// does not have is left to the decoder, which refuses it.
func checkShape(n *yaml.Node, t reflect.Type, at string) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	want := map[reflect.Kind]string{reflect.String: "!!str", reflect.Bool: "!!bool", reflect.Int: "!!int",
		reflect.Struct: "!!map", reflect.Map: "!!map", reflect.Slice: "!!seq"}
	kind := map[reflect.Kind]yaml.Kind{reflect.Struct: yaml.MappingNode, reflect.Map: yaml.MappingNode, reflect.Slice: yaml.SequenceNode}
	if k, ok := kind[t.Kind()]; ok && n.Kind != k || !ok && n.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: %s is not a %s", n.Line, label(at), shapeName(t.Kind()))
	}
	switch t.Kind() {
	case reflect.Float64:
		if n.Tag != "!!float" && n.Tag != "!!int" {
			return fmt.Errorf("line %d: %s is not a number", n.Line, label(at))
		}
		return nil
	case reflect.String, reflect.Bool, reflect.Int:
		if n.Tag != want[t.Kind()] {
			return fmt.Errorf("line %d: %s is not a %s", n.Line, label(at), kindName(t.Kind()))
		}
		return nil
	case reflect.Slice:
		if n.Tag != want[t.Kind()] {
			return fmt.Errorf("line %d: %s is not a list", n.Line, label(at))
		}
		for _, c := range n.Content {
			if err := checkShape(c, t.Elem(), at+"[]"); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		if n.Tag != want[t.Kind()] {
			return fmt.Errorf("line %d: %s is not a mapping", n.Line, label(at))
		}
		for i := 1; i < len(n.Content); i += 2 {
			if err := checkShape(n.Content[i], t.Elem(), at+"{}"); err != nil {
				return err
			}
		}
		return nil
	case reflect.Struct:
		if n.Tag != want[t.Kind()] {
			return fmt.Errorf("line %d: %s is not a mapping", n.Line, label(at))
		}
		values := map[string]*yaml.Node{}
		for i := 0; i < len(n.Content); i += 2 {
			values[n.Content[i].Value] = n.Content[i+1]
		}
		for f := range t.Fields() {
			name := jsonName(f)
			v, ok := values[name]
			if !ok {
				if !optional(f) {
					return fmt.Errorf("line %d: %s has no key %s", n.Line, label(at), name)
				}
				continue
			}
			if err := checkShape(v, f.Type, strings.TrimPrefix(at+"."+name, ".")); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("spec: the kind %s at %s has no shape check", t.Kind(), label(at))
}

// optional reports whether the struct field f may be left out: its JSON
// name has the omitzero option.
func optional(f reflect.StructField) bool {
	_, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
	return slices.Contains(strings.Split(opts, ","), "omitzero")
}

// shapeName names the shape a value of kind k has, for a message.
func shapeName(k reflect.Kind) string {
	switch k {
	case reflect.Struct, reflect.Map:
		return "mapping"
	case reflect.Slice:
		return "list"
	case reflect.Float64:
		return "number"
	}
	return kindName(k)
}

// kindName names a scalar kind for a message.
func kindName(k reflect.Kind) string {
	switch k {
	case reflect.Bool:
		return "boolean"
	case reflect.Int:
		return "whole number"
	}
	return "string"
}

// label names the path at in a message; the empty path is the file.
func label(at string) string {
	if at == "" {
		return "the file"
	}
	return at
}

// setDefaults fills the values the spec may leave out and a validator
// reads: a repo's dir is its URL's base name without .git.
func (p *Project) setDefaults() {
	for i, r := range p.Repos {
		if r.Dir == "" {
			p.Repos[i].Dir = strings.TrimSuffix(path.Base(r.URL), ".git")
		}
	}
}

// clipLen is the most bytes of an input a message quotes.
const clipLen = 64

// clip returns s cut to clipLen bytes, so a message about a long input
// stays short.
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
