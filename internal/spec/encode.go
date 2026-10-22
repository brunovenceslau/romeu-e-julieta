// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"
)

// EncodeProject writes p as YAML, the one writer of a project spec, so
// that what it writes DecodeProject reads back as p (I11).
func EncodeProject(p Project) ([]byte, error) { return encode(&p) }

// EncodeHostSettings writes h as YAML, so that what it writes
// DecodeHostSettings reads back as h (I11).
func EncodeHostSettings(h HostSettings) ([]byte, error) { return encode(&h) }

// encode writes v, a pointer to a format's type, with go.yaml.in/yaml/v3,
// from a node tree this function builds by its yaml tags: the
// marshaller's own Encode reads its output back, and for some strings
// (a newline then a digit) it writes a block scalar it cannot read. A
// string that holds a control character, has space at either end, or
// is "<<" (which reads back as a merge key) is written double-quoted;
// the marshaller quotes any other string that would read back as
// another type.
func encode(v any) ([]byte, error) {
	n, err := toNode(reflect.ValueOf(v))
	if err != nil {
		return nil, err
	}
	out, err := yaml.Marshal(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{n}})
	if err != nil {
		return nil, fmt.Errorf("spec: encode: %w", err)
	}
	return out, nil
}

// toNode returns the node of v, a value of a format's type.
func toNode(v reflect.Value) (*yaml.Node, error) {
	for v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.String:
		return str(v.String()), nil
	case reflect.Bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(v.Bool())}, nil
	case reflect.Int:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.FormatInt(v.Int(), 10)}, nil
	case reflect.Float64:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: floatText(v.Float())}, nil
	case reflect.Slice:
		n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for i := range v.Len() {
			c, err := toNode(v.Index(i))
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, c)
		}
		return n, nil
	case reflect.Map:
		n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		keys := v.MapKeys()
		slices.SortFunc(keys, func(a, b reflect.Value) int { return strings.Compare(a.String(), b.String()) })
		for _, k := range keys {
			c, err := toNode(v.MapIndex(k))
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, str(k.String()), c)
		}
		return n, nil
	case reflect.Struct:
		n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for i, f := range slices.Collect(v.Type().Fields()) {
			name, opts, _ := strings.Cut(f.Tag.Get("yaml"), ",")
			if name == "-" || (opts == "omitempty" && v.Field(i).IsZero()) {
				continue
			}
			c, err := toNode(v.Field(i))
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, str(name), c)
		}
		return n, nil
	}
	return nil, fmt.Errorf("spec: encode: the kind %s has no YAML form", v.Kind())
}

// str returns the node of the string s, double-quoted when encode says.
func str(s string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
	if s == "<<" || hasControl(s) || strings.TrimFunc(s, unicode.IsSpace) != s {
		n.Style = yaml.DoubleQuotedStyle
	}
	return n
}

// floatText writes a number as YAML reads it back: the shortest form,
// and .nan and .inf for the values JSON has no form for.
func floatText(f float64) string {
	switch {
	case math.IsNaN(f):
		return ".nan"
	case math.IsInf(f, 1):
		return ".inf"
	case math.IsInf(f, -1):
		return "-.inf"
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}
