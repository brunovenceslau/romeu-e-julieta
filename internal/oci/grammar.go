// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package oci reads kit and workload descriptors (06 6.1, I28). It owns
// the one descriptor grammar of the product and the normalized capability
// set it yields (REQ-0570), which render calls for local kits; and the
// guarded client that reads a workload's descriptor from its registry
// by digest, anonymously, over HTTPS only, verifying the digest of every
// hop and capping every response (REQ-0515).
//
// Two rules of decoding meet here, and ADR 0041 decides which applies to
// each document. The descriptor is read by the strict subset grammar of
// REQ-0168 whatever its origin, a local file or the annotation of an
// image a third party built: it is the input of a gate, so a key or a
// capability type the grammar does not model fails closed (REQ-0068,
// REQ-0573). The documents of the registry around it (an index, a
// manifest, a token response) are outputs of a tool the product does not
// own, decoded tolerantly and checked strictly (REQ-0170): an unknown
// member is skipped, a member that is read is required and typed, and a
// duplicate member name is refused.
package oci

import (
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// MaxDescriptorSize is the cap of a descriptor read from a file or an
// annotation: the file cap of 07 7.5, step 1 (REQ-0168).
const MaxDescriptorSize = spec.MaxFileSize

// maxDepth and maxNodes bound the nesting and the node count of a
// descriptor, so one under the size cap cannot make the walks long.
const (
	maxDepth = 64
	maxNodes = 65536
)

// ErrInvalid is wrapped by every refusal of the grammar: a document that
// is not a descriptor of the subset the product reads. A caller turns it
// into the sync error of REQ-0573; the text names a line or a path and
// never a value of the document, which can be a secret or an escape
// sequence.
var ErrInvalid = errors.New("oci: not a descriptor the grammar reads")

// invalid returns an ErrInvalid with the reason given.
func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// quote returns s as a short Go string literal, so a name taken from a
// document shows with no control character in it.
func quote(s string) string {
	const limit = 64
	if len(s) > limit {
		cut := limit
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:cut] + "..."
	}
	return strconv.Quote(s)
}

// parseTree reads data, one YAML document (JSON is YAML's flow form, so
// the annotation of an image and a local file take the same road), into
// a tree of maps, lists and scalars. In order: at most MaxDescriptorSize
// bytes of UTF-8; one mapping; no anchor, alias, merge key, tag outside
// YAML's core schema (which refuses an include), duplicate key, null,
// key that is not a string, or nesting past maxDepth (REQ-0168). Why not
// internal/spec's checker: it is unexported there and bound to the types
// of the spec; these are the same rules.
func parseTree(data []byte) (map[string]any, error) {
	if len(data) > MaxDescriptorSize {
		return nil, invalid("the descriptor has %d bytes, over the cap of %d", len(data), MaxDescriptorSize)
	}
	if !utf8.Valid(data) {
		return nil, invalid("the descriptor is not UTF-8")
	}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, invalid("the descriptor is empty")
		}
		return nil, invalid("%s", yamlReason(err))
	}
	if err := dec.Decode(new(yaml.Node)); !errors.Is(err, io.EOF) {
		return nil, invalid("the descriptor holds more than one document")
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, invalid("the descriptor is not one mapping")
	}
	nodes := 0
	root := doc.Content[0]
	if err := checkTree(root, 0, &nodes); err != nil {
		return nil, invalid("%s", err)
	}
	tree, err := toValue(root)
	if err != nil {
		return nil, invalid("%s", err)
	}
	return tree.(map[string]any), nil
}

// jsonTree reads data, one JSON object, into the same tree as parseTree:
// the size, UTF-8 and duplicate-name rules are the decoder's, and the
// tree is held to the nesting, node and null rules of the YAML road.
func jsonTree(data []byte) (map[string]any, error) {
	if len(data) > MaxDescriptorSize {
		return nil, invalid("the descriptor has %d bytes, over the cap of %d", len(data), MaxDescriptorSize)
	}
	var tree map[string]any
	if err := jsonv2.Unmarshal(data, &tree); err != nil {
		return nil, invalid("the descriptor is not one JSON object without a repeated name")
	}
	nodes := 0
	if err := checkJSON(tree, 0, &nodes); err != nil {
		return nil, invalid("%s", err)
	}
	return tree, nil
}

// checkJSON returns why the value v, or one below it, is outside the
// subset: too deep, too many, or null.
func checkJSON(v any, depth int, nodes *int) error {
	if *nodes++; *nodes > maxNodes {
		return fmt.Errorf("the descriptor holds more than %d nodes", maxNodes)
	}
	if depth > maxDepth {
		return fmt.Errorf("the descriptor nests deeper than %d levels", maxDepth)
	}
	switch v := v.(type) {
	case nil:
		return errors.New("a null value; leave the member out instead")
	case map[string]any:
		for _, c := range v {
			if err := checkJSON(c, depth+1, nodes); err != nil {
				return err
			}
		}
	case []any:
		for _, c := range v {
			if err := checkJSON(c, depth+1, nodes); err != nil {
				return err
			}
		}
	}
	return nil
}

// yamlReason says what a parse error of go.yaml.in/yaml/v3 means, with
// its line, without its text, which quotes the document.
func yamlReason(err error) string {
	msg := err.Error()
	what := "the descriptor is not YAML the grammar reads"
	if strings.Contains(msg, "already defined") {
		what = "a key written twice"
	}
	if _, rest, ok := strings.Cut(msg, "line "); ok {
		digits := rest[:len(rest)-len(strings.TrimLeft(rest, "0123456789"))]
		if digits != "" {
			return "line " + digits + ": " + what
		}
	}
	return what
}

// coreTags are the tags of YAML's core schema a value may resolve to.
var coreTags = []string{"!!str", "!!int", "!!bool", "!!float", "!!seq", "!!map"}

// checkTree returns why the node n, or one below it, is outside the
// subset grammar; nodes counts the nodes seen.
func checkTree(n *yaml.Node, depth int, nodes *int) error {
	if *nodes++; *nodes > maxNodes {
		return fmt.Errorf("the descriptor holds more than %d nodes", maxNodes)
	}
	switch {
	case depth > maxDepth:
		return fmt.Errorf("the descriptor nests deeper than %d levels", maxDepth)
	case n.Kind == yaml.AliasNode:
		return fmt.Errorf("line %d: an alias", n.Line)
	case n.Anchor != "":
		return fmt.Errorf("line %d: an anchor", n.Line)
	case n.Tag == "!!null":
		return fmt.Errorf("line %d: a null value; leave the key out instead", n.Line)
	case n.Tag == "!!merge" && n.Kind == yaml.ScalarNode:
		// A merge key is refused below, as a key; a value "<<" is the
		// string, which go.yaml.in/yaml/v3 reads back with this tag.
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
			case seen[k.Value]:
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

// toValue converts a checked node into maps, lists and scalars that
// encoding/json/v2 writes.
func toValue(n *yaml.Node) (any, error) {
	switch n.Kind {
	case yaml.MappingNode:
		m := make(map[string]any, len(n.Content)/2)
		for i := 0; i < len(n.Content); i += 2 {
			v, err := toValue(n.Content[i+1])
			if err != nil {
				return nil, err
			}
			m[n.Content[i].Value] = v
		}
		return m, nil
	case yaml.SequenceNode:
		l := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			v, err := toValue(c)
			if err != nil {
				return nil, err
			}
			l = append(l, v)
		}
		return l, nil
	}
	switch n.Tag {
	case "!!int":
		var i int64
		if err := n.Decode(&i); err != nil {
			return nil, fmt.Errorf("line %d: a whole number out of range", n.Line)
		}
		return i, nil
	case "!!float":
		var f float64
		if err := n.Decode(&f); err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, fmt.Errorf("line %d: a number out of range", n.Line)
		}
		return f, nil
	case "!!bool":
		var b bool
		if err := n.Decode(&b); err != nil {
			return nil, fmt.Errorf("line %d: a boolean the grammar does not read", n.Line)
		}
		return b, nil
	}
	return n.Value, nil
}
