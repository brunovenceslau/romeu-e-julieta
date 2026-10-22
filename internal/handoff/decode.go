// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// ErrInvalid is the error of a handoff file this build does not read:
// it does not decode strictly, breaks a rule of 03 3.10 or passes the
// file cap. A newer version is format-newer (RJ-311) instead.
var ErrInvalid = errors.New("handoff: not a handoff file")

// The lines that open and close the front matter.
const (
	frontOpen  = "---\n"
	frontClose = "\n---\n"
)

// The bounds of the fields of the front matter.
const (
	// maxText is the most bytes of a branch, an upstream, an agent or
	// a project.
	maxText = 256
	// maxRepos is the most repos a front matter lists.
	maxRepos = 64
	// maxCount bounds ahead, behind, dirty and stashes.
	maxCount = 1 << 31
)

var (
	frontKeys = []string{"schema", "kind", "createdAt", "project", "generation", "agent", "repos"}
	repoKeys  = []string{"dir", "branch", "head", "upstream", "ahead", "behind", "dirty", "stashes"}
	// schemaField reads a schema field of the form "handoff/v<n>".
	schemaField = regexp.MustCompile(`^handoff/v([1-9][0-9]{0,8})$`)
	// schemaLine finds a schema field in the text of a front matter that
	// does not parse, block or flow style, before any YAML parse.
	schemaLine = regexp.MustCompile(`schema:[ \t]+["']?handoff/v([1-9][0-9]{0,8})["']?`)
	// objectID is a git object id in lowercase hex, SHA-1 or SHA-256.
	objectID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	// dirForm is the form of a repo dir (03 3.1, REQ-0176).
	dirForm = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)
)

// Check returns why f is no handoff file of the format: a rule of 03
// 3.10 it breaks. A narrative needs its headings in order (REQ-0245),
// facts has no body, and the front matter fields have the forms of the
// format. It does not look at the size of the file.
func (f File) Check() error {
	if f.Schema != Field {
		return fmt.Errorf("%w: the schema \"%s\" is not %s", ErrInvalid, clip(f.Schema), Field)
	}
	if !f.Kind.Valid() {
		return fmt.Errorf("%w: the kind \"%s\" is not clear, final or facts", ErrInvalid, clip(string(f.Kind)))
	}
	if t := f.CreatedAt; t.Location() != time.UTC || !t.Truncate(time.Second).Equal(t) {
		return fmt.Errorf("%w: createdAt %s is not UTC to the second", ErrInvalid, t)
	}
	if _, err := canon.ParseULID(f.Generation); err != nil {
		return fmt.Errorf("%w: the generation is not a lowercase ULID", ErrInvalid)
	}
	for name, v := range map[string]string{"project": f.Project, "agent": f.Agent} {
		if err := text(name, v, false); err != nil {
			return err
		}
	}
	if f.Repos == nil || len(f.Repos) > maxRepos {
		return fmt.Errorf("%w: repos is null or lists more than %d repos", ErrInvalid, maxRepos)
	}
	seen := map[string]bool{}
	for _, r := range f.Repos {
		if err := r.check(); err != nil {
			return err
		}
		if seen[r.Dir] {
			return fmt.Errorf("%w: the repo %s is listed twice", ErrInvalid, clip(r.Dir))
		}
		seen[r.Dir] = true
	}
	if !utf8.ValidString(f.Body) {
		return fmt.Errorf("%w: the body is not UTF-8", ErrInvalid)
	}
	if f.Kind == Facts {
		if f.Body != "" {
			return fmt.Errorf("%w: a facts handoff has no body", ErrInvalid)
		}
		return nil
	}
	return CheckHeadings(f.Kind, f.Body)
}

// check returns why r is no repo of a front matter.
func (r RepoFacts) check() error {
	if !dirForm.MatchString(r.Dir) {
		return fmt.Errorf("%w: the repo dir \"%s\" is not a repo dir of 03 3.1", ErrInvalid, clip(r.Dir))
	}
	for name, v := range map[string]string{"branch": r.Branch, "upstream": r.Upstream} {
		if err := text(name, v, true); err != nil {
			return err
		}
	}
	if r.Head != "" && !objectID.MatchString(r.Head) {
		return fmt.Errorf("%w: the head \"%s\" is not a full object id", ErrInvalid, clip(r.Head))
	}
	for name, n := range map[string]int{"ahead": r.Ahead, "behind": r.Behind, "dirty": r.Dirty, "stashes": r.Stashes} {
		if n < 0 || n > maxCount {
			return fmt.Errorf("%w: %s is %d, outside 0 to %d", ErrInvalid, name, n, maxCount)
		}
	}
	return nil
}

// text returns why v is no value of the field name: not one line of
// printable text, or over maxText bytes; empty only when empty is set.
func text(name, v string, empty bool) error {
	switch {
	case v == "" && !empty:
		return fmt.Errorf("%w: %s is empty", ErrInvalid, name)
	case len(v) > maxText:
		return fmt.Errorf("%w: %s has %d bytes, over %d", ErrInvalid, name, len(v), maxText)
	case !utf8.ValidString(v) || strings.ContainsFunc(v, func(r rune) bool { return !unicode.IsPrint(r) }):
		return fmt.Errorf("%w: %s is not one line of printable text", ErrInvalid, name)
	}
	return nil
}

// CheckHeadings returns why body is no narrative of kind k: a heading
// line "## <Name>" of Headings(k) that is missing or out of order
// (REQ-0245). It matches lines, not Markdown: a heading inside a fenced
// block counts.
func CheckHeadings(k Kind, body string) error {
	want := Headings(k)
	if want == nil {
		return fmt.Errorf("%w: the kind %s is no narrative", ErrInvalid, clip(string(k)))
	}
	i := 0
	for line := range strings.Lines(body) {
		if i < len(want) && strings.TrimRight(line, " \t\r\n") == "## "+want[i] {
			i++
		}
	}
	if i < len(want) {
		return fmt.Errorf("%w: a %s handoff needs the heading \"## %s\" (in the order %s)", ErrInvalid, k, want[i],
			"## "+strings.Join(want, ", ## "))
	}
	return nil
}

// Encode returns the file of f: the front matter between two "---"
// lines, then the body as it is. A file that breaks a rule of 03 3.10,
// or that passes memstore.MaxFileSize, is an error.
func Encode(f File) ([]byte, error) {
	if err := f.Check(); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString(frontOpen)
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := errors.Join(enc.Encode(f.Front), enc.Close()); err != nil {
		return nil, fmt.Errorf("handoff: encode: %w", err)
	}
	b.WriteString(frontClose[1:])
	b.WriteString(f.Body)
	if b.Len() > memstore.MaxFileSize {
		return nil, fmt.Errorf("%w: the file has %d bytes, over %d", ErrInvalid, b.Len(), memstore.MaxFileSize)
	}
	return b.Bytes(), nil
}

// Decode reads the file of a handoff strictly (03, opening, REQ-0168):
// at most memstore.MaxFileSize bytes of UTF-8; a front matter between a
// first line "---" and the next, of one YAML mapping with no anchor,
// alias, merge key, tag outside YAML's core schema (an include among
// them) or duplicate key, nested at most maxDepth levels; then
// go.yaml.in/yaml/v3 into Front, every key of v1 required and a key
// this build does not know skipped, in the front matter and in a repo,
// as a newer writer of the same version may add one (REQ-0163); then
// the rules of 03 3.10 (File.Check). The schema field is read from the
// parsed mapping before the keys are judged, so a newer version is
// format-newer (RJ-311) though it holds keys v1 does not know; a front matter that does not parse, or that the subset grammar
// refuses, is format-newer too when the text of its schema line names a
// newer version. Every other refusal wraps ErrInvalid.
func Decode(data []byte) (File, error) {
	if len(data) > memstore.MaxFileSize {
		return File{}, fmt.Errorf("%w: the file has %d bytes, over %d", ErrInvalid, len(data), memstore.MaxFileSize)
	}
	if !utf8.Valid(data) {
		return File{}, fmt.Errorf("%w: the file is not UTF-8", ErrInvalid)
	}
	rest, ok := bytes.CutPrefix(data, []byte(frontOpen))
	if !ok {
		return File{}, fmt.Errorf("%w: the file does not start with a line ---", ErrInvalid)
	}
	var front, body []byte
	if b, ok := bytes.CutPrefix(rest, []byte(frontClose[1:])); ok {
		body = b
	} else {
		i := bytes.Index(rest, []byte(frontClose))
		if i < 0 {
			return File{}, fmt.Errorf("%w: the front matter has no closing line ---", ErrInvalid)
		}
		front, body = rest[:i+1], rest[i+len(frontClose):]
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(front, &doc); err != nil {
		return File{}, refused(front, err)
	}
	if err := checkNode(&doc); err != nil {
		return File{}, refused(front, err)
	}
	if err := checkVersion(doc.Content[0]); err != nil {
		return File{}, err
	}
	if err := checkKeys(doc.Content[0]); err != nil {
		return File{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	var f File
	dec := yaml.NewDecoder(bytes.NewReader(front))
	if err := dec.Decode(&f.Front); err != nil {
		return File{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := dec.Decode(new(yaml.Node)); !errors.Is(err, io.EOF) {
		return File{}, fmt.Errorf("%w: the front matter holds more than one document", ErrInvalid)
	}
	f.Body = string(body)
	if err := f.Check(); err != nil {
		return File{}, err
	}
	return f, nil
}

// refused returns the error of a front matter that does not parse or
// that the subset grammar refuses for the reason err: format-newer when
// its schema line names a newer version, ErrInvalid otherwise.
func refused(front []byte, err error) error {
	if sub := schemaLine.FindSubmatch(front); sub != nil {
		if n, aerr := strconv.Atoi(string(sub[1])); aerr == nil && n > Version {
			return manifest.FormatNewer.New(fmt.Sprintf("the handoff file is handoff/v%d, newer than %s, the version this build reads", n, Field))
		}
	}
	return fmt.Errorf("%w: %w", ErrInvalid, err)
}

// coreTags are the tags of YAML's core schema that a front matter may
// resolve to.
var coreTags = []string{"!!str", "!!int", "!!bool", "!!null", "!!float", "!!timestamp", "!!seq", "!!map"}

// maxDepth bounds the nesting of a front matter: the mapping, the
// repos list, a repo mapping and its values.
const maxDepth = 5

// checkNode returns why doc, a decoded front matter, is outside the
// subset grammar: it is not one mapping, or a node in it has an anchor,
// is an alias, is a merge key, has a tag outside coreTags, repeats a key
// of its mapping, or nests too deep.
func checkNode(doc *yaml.Node) error {
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return errors.New("the front matter is not one mapping")
	}
	return checkTree(doc.Content[0], 0)
}

// checkTree returns why the node n or one below it is outside the
// subset grammar.
func checkTree(n *yaml.Node, depth int) error {
	switch {
	case depth > maxDepth:
		return errors.New("the front matter nests too deep")
	case n.Kind == yaml.AliasNode:
		return errors.New("the front matter holds an alias")
	case n.Anchor != "":
		return errors.New("the front matter holds an anchor")
	case n.Tag == "!!merge" || (n.Kind == yaml.ScalarNode && n.Value == "<<" && n.Style == 0):
		return errors.New("the front matter holds a merge key")
	case !slices.Contains(coreTags, n.Tag):
		return fmt.Errorf("the front matter holds the tag \"%s\", outside YAML's core schema", clip(n.Tag))
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Kind != yaml.ScalarNode {
				return errors.New("the front matter holds a key that is not a scalar")
			}
			if seen[k.Value] {
				return fmt.Errorf("the key \"%s\" is written twice", clip(k.Value))
			}
			seen[k.Value] = true
		}
	}
	for _, c := range n.Content {
		if err := checkTree(c, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// checkKeys returns why the front matter m lacks a key of frontKeys, and
// a repo lacks one of repoKeys; a key in none of them is skipped.
func checkKeys(m *yaml.Node) error {
	if err := requireKeys(m, frontKeys, "the front matter"); err != nil {
		return err
	}
	for i := 0; i < len(m.Content); i += 2 {
		if m.Content[i].Value != "repos" {
			continue
		}
		list := m.Content[i+1]
		if list.Kind != yaml.SequenceNode {
			if list.Tag == "!!null" {
				return nil // File.Check refuses a null list
			}
			return errors.New("repos is not a list")
		}
		for _, r := range list.Content {
			if r.Kind != yaml.MappingNode {
				return errors.New("a repo is not a mapping")
			}
			if err := requireKeys(r, repoKeys, "a repo"); err != nil {
				return err
			}
		}
	}
	return nil
}

// requireKeys returns why the mapping m lacks a key of want (its keys
// are distinct, checkTree having passed).
func requireKeys(m *yaml.Node, want []string, what string) error {
	have := map[string]bool{}
	for i := 0; i < len(m.Content); i += 2 {
		have[m.Content[i].Value] = true
	}
	for _, key := range want {
		if !have[key] {
			return fmt.Errorf("the key %s is missing from %s", key, what)
		}
	}
	return nil
}

// checkVersion reads the schema field of the mapping m: a newer
// version is format-newer (RJ-311), and any other value than Field
// wraps ErrInvalid.
func checkVersion(m *yaml.Node) error {
	for i := 0; i < len(m.Content); i += 2 {
		if m.Content[i].Value != "schema" {
			continue
		}
		v := m.Content[i+1]
		sub := schemaField.FindStringSubmatch(v.Value)
		if sub == nil || v.Kind != yaml.ScalarNode {
			return fmt.Errorf("%w: the schema \"%s\" is not handoff/v<n>", ErrInvalid, clip(v.Value))
		}
		if n, _ := strconv.Atoi(sub[1]); n > Version {
			return manifest.FormatNewer.New(fmt.Sprintf("the handoff file is %s, newer than %s, the version this build reads", v.Value, Field))
		}
		return nil
	}
	return fmt.Errorf("%w: no schema field", ErrInvalid)
}

// clipLen is the most bytes of an input that an error message quotes.
const clipLen = 64

// clip returns s cut to clipLen bytes and escaped with termsafe, so an
// error about a long input stays short and a name an agent wrote cannot
// forge a line of the output.
func clip(s string) string {
	if len(s) > clipLen {
		s = s[:clipLen] + "..."
	}
	return termsafe.Escape(s)
}
