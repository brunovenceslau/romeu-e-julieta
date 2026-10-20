// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package schemagen is the JSON Schema generator of 02 2.1 and 12 12.3:
// it reads a Go type by reflection and writes the JSON Schema 2020-12
// document of the format it decodes (03 (opening)). A format joins the
// generator through Register, called from an init, and tools/ci writes
// schemas/<name>.json for each registered format ("go generate ./...")
// and fails a committed schema that differs (tools/ci schema).
//
// The document follows the JSON names of encoding/json/v2, the decoder
// of every owned format (REQ-0160), and its strictness: an object
// admits no member its struct does not name. Every format has a
// "schema" string field, which the document holds as a constant
// (REQ-0161). The rules of internal/spec/rules.go join the documents
// with the task that writes them.
//
// Of the interfaces encoding/json/v2 reads, a type that implements
// encoding.TextMarshaler or encoding.TextUnmarshaler is a string, and
// one with its own JSON methods (MarshalJSON, UnmarshalJSON,
// MarshalJSONTo, UnmarshalJSONFrom) is refused, since its form is its
// code's. time.Time is a date-time string, []byte and [N]byte (named or
// not) a base64 string, and time.Duration, which v2 has no default form
// for, is refused. A pointer field has the schema of what it points to
// and does not admit null: the decoder reads null as the zero value,
// and a document that means "absent" leaves the member out.
package schemagen

import (
	"bytes"
	"encoding"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Draft is the JSON Schema dialect of every generated document.
const Draft = "https://json-schema.org/draft/2020-12/schema"

// Schema is one format: its schema name and version, such as
// "manifest.v1", and the Go type its documents decode into.
type Schema struct {
	Name string
	Type reflect.Type
}

// Path is the slash-separated path of the committed schema, from the
// root of the repository.
func (s Schema) Path() string { return "schemas/" + s.Name + ".json" }

// Field is the value of the format's "schema" field (REQ-0161),
// "<name>/v<n>": the name with its last "." replaced by "/".
func (s Schema) Field() string {
	i := strings.LastIndex(s.Name, ".")
	return s.Name[:i] + "/" + s.Name[i+1:]
}

// Source names the Go type the document is generated from, for the
// generated-file header (12 12.3).
func (s Schema) Source() string { return s.Type.PkgPath() + "." + s.Type.Name() }

// validName is the form of a schema name: a lowercase name, then the
// version.
var validName = regexp.MustCompile(`^[a-z][a-z0-9-]*\.v[1-9][0-9]*$`)

// registry holds the registered formats.
var registry []Schema

// Register adds a format to the generator. A name that is not of the
// form "<name>.v<n>", a name registered already, or a type that is not
// a named struct is a programming error, and Register panics: it runs
// from an init, so the binary does not start.
func Register(s Schema) {
	registry = add(registry, s)
}

// add returns list with s added, or panics as Register says.
func add(list []Schema, s Schema) []Schema {
	if !validName.MatchString(s.Name) {
		panic(fmt.Sprintf("schemagen: %q is not a schema name of the form <name>.v<n>", s.Name))
	}
	if slices.ContainsFunc(list, func(o Schema) bool { return o.Name == s.Name }) {
		panic(fmt.Sprintf("schemagen: %q is registered twice", s.Name))
	}
	if s.Type == nil || s.Type.Kind() != reflect.Struct || s.Type.Name() == "" {
		panic(fmt.Sprintf("schemagen: the type of %q is not a named struct", s.Name))
	}
	return append(list, s)
}

// Registered returns the registered formats, sorted by name.
func Registered() []Schema {
	out := slices.Clone(registry)
	slices.SortFunc(out, func(a, b Schema) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// Generate returns the document of s: the dialect, the title, comment
// as "$comment" (the generated-file header of 12 12.3, since JSON has
// no comment syntax), and the schema of its type. Each member is on a
// line of its own and the members of an object are sorted, so the
// document is the same on every run (12 12.3). A type the generator
// cannot express is an error that names the field.
func Generate(s Schema, comment string) ([]byte, error) {
	g := generator{seen: map[reflect.Type]bool{}}
	doc, err := g.of(s.Type, s.Type.Name())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.Name, err)
	}
	prop, _ := doc["properties"].(map[string]any)
	field, ok := prop["schema"].(map[string]any)
	if !ok || field["type"] != "string" {
		return nil, fmt.Errorf("%s: %s has no schema string field (REQ-0161)", s.Name, s.Type.Name())
	}
	field["const"] = s.Field()
	doc["$schema"] = Draft
	doc["title"] = s.Name
	if comment != "" {
		doc["$comment"] = comment
	}
	// Two-space indent, keys sorted, <, > and & as they are, U+2028 and
	// U+2029 escaped, and a final newline: the committed form.
	var b bytes.Buffer
	if err := jsonv2.MarshalWrite(&b, doc, jsonv2.Deterministic(true),
		jsontext.Multiline(true), jsontext.WithIndent("  "), jsontext.EscapeForJS(true)); err != nil {
		return nil, fmt.Errorf("%s: %w", s.Name, err)
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}

// generator walks one type; seen holds the named types on the path
// from the root, so a recursive type (a struct, or a named map, slice,
// array or pointer) is an error rather than an endless walk.
type generator struct {
	seen map[reflect.Type]bool
}

// The types the generator knows by name.
var (
	timeType     = reflect.TypeFor[time.Time]()
	durationType = reflect.TypeFor[time.Duration]()
	// jsonMethods are the interfaces whose methods give a type a JSON
	// form of its own.
	jsonMethods = []reflect.Type{
		reflect.TypeFor[jsonv2.Marshaler](), reflect.TypeFor[jsonv2.Unmarshaler](),
		reflect.TypeFor[jsonv2.MarshalerTo](), reflect.TypeFor[jsonv2.UnmarshalerFrom](),
	}
	textMethods = []reflect.Type{reflect.TypeFor[encoding.TextMarshaler](), reflect.TypeFor[encoding.TextUnmarshaler]()}
)

// implementsAny reports whether t or a pointer to it implements one of
// ifaces.
func implementsAny(t reflect.Type, ifaces []reflect.Type) bool {
	return slices.ContainsFunc(ifaces, func(i reflect.Type) bool {
		return t.Implements(i) || reflect.PointerTo(t).Implements(i)
	})
}

// errUnsupported ends the error of a type or a tag the generator cannot
// express.
const errUnsupported = "is not supported by the generator"

// jsonOptions are the options of a json tag the generator reads; any
// other option, such as string or case, changes the encoding in a way
// it does not express, and is an error.
var jsonOptions = []string{"omitempty", "omitzero", "embed"}

// of returns the schema of t, reached at where (a Go path such as
// "Manifest.Repos[]").
func (g generator) of(t reflect.Type, where string) (map[string]any, error) {
	switch {
	case t == timeType:
		return map[string]any{"type": "string", "format": "date-time"}, nil
	case t == durationType:
		return nil, fmt.Errorf("%s: time.Duration, which encoding/json/v2 has no default form for, %s", where, errUnsupported)
	case implementsAny(t, jsonMethods):
		return nil, fmt.Errorf("%s: a type with its own JSON methods %s", where, errUnsupported)
	case implementsAny(t, textMethods):
		return map[string]any{"type": "string"}, nil
	case (t.Kind() == reflect.Slice || t.Kind() == reflect.Array) && t.Elem().Kind() == reflect.Uint8:
		return map[string]any{"type": "string", "contentEncoding": "base64"}, nil
	}
	if t.Name() != "" && slices.Contains([]reflect.Kind{reflect.Map, reflect.Slice, reflect.Array, reflect.Pointer}, t.Kind()) {
		if g.seen[t] {
			return nil, fmt.Errorf("%s: the recursive type %s %s", where, t, errUnsupported)
		}
		g.seen[t] = true
		defer delete(g.seen, t)
	}
	switch t.Kind() {
	case reflect.Bool:
		return map[string]any{"type": "boolean"}, nil
	case reflect.String:
		return map[string]any{"type": "string"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return map[string]any{"type": "integer"}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer", "minimum": 0}, nil
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}, nil
	case reflect.Pointer:
		return g.of(t.Elem(), where)
	case reflect.Interface:
		if t.NumMethod() != 0 {
			return nil, fmt.Errorf("%s: an interface with methods %s", where, errUnsupported)
		}
		return map[string]any{}, nil
	case reflect.Slice, reflect.Array:
		items, err := g.of(t.Elem(), where+"[]")
		if err != nil {
			return nil, err
		}
		out := map[string]any{"type": "array", "items": items}
		if t.Kind() == reflect.Array {
			out["minItems"], out["maxItems"] = t.Len(), t.Len()
		}
		return out, nil
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("%s: a map whose key is not a string %s", where, errUnsupported)
		}
		values, err := g.of(t.Elem(), where+"[key]")
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "object", "additionalProperties": values}, nil
	case reflect.Struct:
		return g.object(t, where)
	default:
		return nil, fmt.Errorf("%s: the kind %s %s", where, t.Kind(), errUnsupported)
	}
}

// object returns the schema of the struct t: its members by JSON name,
// those without omitempty or omitzero required, and no other member.
func (g generator) object(t reflect.Type, where string) (map[string]any, error) {
	if g.seen[t] {
		return nil, fmt.Errorf("%s: the recursive type %s %s", where, t, errUnsupported)
	}
	g.seen[t] = true
	defer delete(g.seen, t)
	props := map[string]any{}
	required := []string{}
	if err := g.fields(t, where, props, &required); err != nil {
		return nil, err
	}
	out := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		slices.Sort(required)
		out["required"] = required
	}
	return out, nil
}

// fields adds the members of the struct t to props and the names of
// the required ones to required. A Go embedded struct without a JSON
// name, or a field with the embed option, adds its members in place,
// as encoding/json/v2 decodes them.
func (g generator) fields(t reflect.Type, where string, props map[string]any, required *[]string) error {
	for f := range t.Fields() {
		if !f.IsExported() && !f.Anonymous {
			continue
		}
		tag, hasTag := f.Tag.Lookup("json")
		if tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		var options []string
		if opts != "" {
			options = strings.Split(opts, ",")
		}
		for _, o := range options {
			if !slices.Contains(jsonOptions, o) {
				return fmt.Errorf("%s.%s: the json option %q %s", where, f.Name, o, errUnsupported)
			}
		}
		ft := f.Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		embed := slices.Contains(options, "embed") || (f.Anonymous && name == "" && ft.Kind() == reflect.Struct)
		if embed {
			if ft.Kind() != reflect.Struct {
				return fmt.Errorf("%s.%s: embed on a field that is not a struct %s", where, f.Name, errUnsupported)
			}
			if err := g.fields(ft, where, props, required); err != nil {
				return err
			}
			continue
		}
		if !f.IsExported() {
			continue
		}
		if !hasTag || name == "" {
			name = f.Name
		}
		if _, dup := props[name]; dup {
			return fmt.Errorf("%s.%s: the JSON name %q is used twice", where, f.Name, name)
		}
		s, err := g.of(f.Type, where+"."+f.Name)
		if err != nil {
			return err
		}
		props[name] = s
		if !slices.Contains(options, "omitempty") && !slices.Contains(options, "omitzero") {
			*required = append(*required, name)
		}
	}
	return nil
}
