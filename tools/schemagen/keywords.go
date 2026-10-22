// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schemagen

import (
	"fmt"
	"slices"
	"strings"
)

// Keyword adds JSON Schema keywords to the place of a document a path
// names: the constraints of a format's validation rules (REQ-0172, for
// internal/spec/rules.go). Path is a path of JSON names from the top of
// the format: "a.b" is a member, "a[]" each element of a list, "a[0]"
// its first element, "a{}" each value of a map and "a{key}" each key;
// "[0]" and "{key}" end a path. A path that passes a $ref follows it,
// so a member of a recursive type is named by its first place.
type Keyword struct {
	Path   string
	Values map[string]any
}

// addKeywords adds each keyword of kws to doc, the generated schema of a
// format. A keyword a place holds already is added beside it, in an
// allOf, so both apply. A path that names no place of doc is an error.
func addKeywords(doc map[string]any, kws []Keyword) error {
	for _, kw := range kws {
		node, last, err := place(doc, kw.Path)
		if err != nil {
			return fmt.Errorf("the keyword path %q: %w", kw.Path, err)
		}
		switch last {
		case "[0]":
			if node["type"] != "array" {
				return fmt.Errorf("the keyword path %q: [0] of a value that is not a list", kw.Path)
			}
			// Inside allOf, so that the items of the list still apply to
			// the first element too.
			node["allOf"] = append(allOf(node), map[string]any{"prefixItems": []any{kw.Values}})
		case "{key}":
			if node["type"] != "object" || node["properties"] != nil {
				return fmt.Errorf("the keyword path %q: {key} of a value that is not a map", kw.Path)
			}
			names, _ := node["propertyNames"].(map[string]any)
			if names == nil {
				names = map[string]any{}
				node["propertyNames"] = names
			}
			merge(names, kw.Values)
		default:
			merge(node, kw.Values)
		}
	}
	return nil
}

// allOf returns the allOf list of node, or none.
func allOf(node map[string]any) []any {
	list, _ := node["allOf"].([]any)
	return list
}

// merge adds values to node; a keyword node holds already is added in
// an allOf.
func merge(node, values map[string]any) {
	for _, k := range sortedKeys(values) {
		if _, ok := node[k]; ok {
			node["allOf"] = append(allOf(node), map[string]any{k: values[k]})
			continue
		}
		node[k] = values[k]
	}
}

// sortedKeys returns the keys of m, sorted, so the document is the same
// on every run.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// place returns the schema the path p names in doc and the suffix that
// ends p when it is "[0]" or "{key}", in which case the schema is that
// of the list or the map.
func place(doc map[string]any, p string) (map[string]any, string, error) {
	node := doc
	rest := p
	for rest != "" {
		var err error
		if node, err = follow(doc, node); err != nil {
			return nil, "", err
		}
		switch {
		case strings.HasPrefix(rest, "[0]") || strings.HasPrefix(rest, "{key}"):
			last := rest[:strings.IndexAny(rest, "]}")+1]
			if rest != last {
				return nil, "", fmt.Errorf("%s does not end the path", last)
			}
			return node, last, nil
		case strings.HasPrefix(rest, "[]"):
			node, rest = child(node, "items", ""), rest[2:]
			if node == nil {
				return nil, "", fmt.Errorf("[] of a value that is not a list")
			}
		case strings.HasPrefix(rest, "{}"):
			if node["properties"] != nil {
				return nil, "", fmt.Errorf("{} of a value that is not a map")
			}
			node, rest = child(node, "additionalProperties", ""), rest[2:]
			if node == nil {
				return nil, "", fmt.Errorf("{} of a value that is not a map")
			}
		default:
			rest = strings.TrimPrefix(rest, ".")
			end := strings.IndexAny(rest, ".[{")
			if end < 0 {
				end = len(rest)
			}
			name := rest[:end]
			node, rest = child(node, "properties", name), rest[end:]
			if node == nil {
				return nil, "", fmt.Errorf("no member %q", name)
			}
		}
	}
	node, err := follow(doc, node)
	return node, "", err
}

// child returns node[key], or node[key][name] when name is not empty,
// as a schema object, or nil.
func child(node map[string]any, key, name string) map[string]any {
	c, _ := node[key].(map[string]any)
	if name == "" || c == nil {
		return c
	}
	m, _ := c[name].(map[string]any)
	return m
}

// follow returns the schema node's $ref names in doc, a pointer that
// the generator wrote, or node when it holds none.
func follow(doc, node map[string]any) (map[string]any, error) {
	ref, ok := node["$ref"].(string)
	if !ok {
		return node, nil
	}
	target := doc
	for _, tok := range strings.Split(strings.TrimPrefix(ref, "#"), "/")[1:] {
		next, _ := target[unescapeToken(tok)].(map[string]any)
		if next == nil {
			return nil, fmt.Errorf("the $ref %q names no schema", ref)
		}
		target = next
	}
	return target, nil
}

// pointerToken escapes a member name as a JSON pointer token (RFC 6901).
func pointerToken(name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(name, "~", "~0"), "/", "~1")
}

// unescapeToken reverses pointerToken.
func unescapeToken(tok string) string {
	return strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")
}
