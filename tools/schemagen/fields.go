// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schemagen

import (
	"maps"
	"reflect"
	"strconv"
)

// Field is one member of a format, at any depth, for the field table of
// its reference page (S10, 12 12.3).
type Field struct {
	// Path is the member's path of JSON names, in the syntax of Keyword:
	// "repos[].dir".
	Path string
	// Type says the member's type in words, such as "list of string".
	Type string
	// Required says the member may not be left out.
	Required bool
	// Field is the struct field the member comes from, with its tags,
	// and Owner the struct type that declares it, whose source holds
	// the field's doc comment.
	Field reflect.StructField
	Owner reflect.Type
}

// Fields returns the members of the format type t, in the order of the
// types, each followed by the members below it. A struct type reached
// again on its own path is listed once, at its first place, and its
// later places say "as <path>".
func Fields(t reflect.Type) ([]Field, error) {
	var out []Field
	var visit func(t reflect.Type, path string, first map[reflect.Type]string) error
	visit = func(t reflect.Type, path string, first map[reflect.Type]string) error {
		ms, err := members(t, t.Name())
		if err != nil {
			return err
		}
		for _, m := range ms {
			p := m.Name
			if path != "" {
				p = path + "." + m.Name
			}
			out = append(out, Field{Path: p, Type: describe(m.Field.Type, first), Required: m.Required, Field: m.Field, Owner: m.Owner})
			inner, suffix := m.Field.Type, ""
			for {
				for inner.Kind() == reflect.Pointer {
					inner = inner.Elem()
				}
				if !isContainer(inner) {
					break
				}
				if inner.Kind() == reflect.Map {
					suffix += "{}"
				} else {
					suffix += "[]"
				}
				inner = inner.Elem()
			}
			if inner.Kind() != reflect.Struct || isScalarStruct(inner) {
				continue
			}
			if _, ok := first[inner]; ok {
				continue
			}
			if err := visit(inner, p+suffix, withPlace(first, inner, p+suffix)); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(t, "", map[reflect.Type]string{t: ""}); err != nil {
		return nil, err
	}
	return out, nil
}

// withPlace returns a copy of first with t placed at p.
func withPlace(first map[reflect.Type]string, t reflect.Type, p string) map[reflect.Type]string {
	out := make(map[reflect.Type]string, len(first)+1)
	maps.Copy(out, first)
	out[t] = p
	return out
}

// isContainer reports whether t is a list or a map whose form is not a
// string, as a []byte's is.
func isContainer(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		return t.Elem().Kind() != reflect.Uint8 && !isScalarStruct(t)
	case reflect.Map:
		return true
	}
	return false
}

// isScalarStruct reports whether t has a form of its own: a time or a
// text form, as of says.
func isScalarStruct(t reflect.Type) bool {
	return t == timeType || implementsAny(t, textMethods)
}

// describe says the type t in words, as of writes it; a struct type that
// first places names a place already is "object, as <path>".
func describe(t reflect.Type, first map[reflect.Type]string) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch {
	case t == timeType:
		return "string (date-time)"
	case implementsAny(t, textMethods):
		return "string"
	case (t.Kind() == reflect.Slice || t.Kind() == reflect.Array) && t.Elem().Kind() == reflect.Uint8:
		return "string (base64)"
	}
	switch t.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Interface:
		return "any"
	case reflect.Slice:
		return "list of " + describe(t.Elem(), first)
	case reflect.Array:
		return "list of " + strconv.Itoa(t.Len()) + " " + describe(t.Elem(), first)
	case reflect.Map:
		return "map of string to " + describe(t.Elem(), first)
	case reflect.Struct:
		if p, ok := first[t]; ok {
			if p == "" {
				return "object, as the document"
			}
			return "object, as " + p
		}
		return "object"
	}
	return t.Kind().String()
}
