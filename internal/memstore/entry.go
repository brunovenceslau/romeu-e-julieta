// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memstore

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
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// Version is the version of the format memory-entry, the n of the
// schema field "memory-entry/v<n>" (REQ-0161). A reader refuses a newer
// one with format-newer (RJ-311).
const Version = 1

// Field is the value of the schema field this build writes and reads.
var Field = "memory-entry/v" + strconv.Itoa(Version)

// The statuses of an entry (03 3.9).
const (
	StatusOpen = "open"
	StatusDone = "done"
)

// The five type tags, the closed set of tags with behaviour (03 3.9,
// REQ-0242): the four kinds of note the agent's own memory records,
// and the handoff a session leaves for the next.
const (
	TypeUser      = "user"
	TypeFeedback  = "feedback"
	TypeProject   = "project"
	TypeReference = "reference"
	TypeHandoff   = "handoff"
)

// TypeTags are the five type tags, in the order of 03 3.9.
var TypeTags = []string{TypeUser, TypeFeedback, TypeProject, TypeReference, TypeHandoff}

// NoType is the type the index line shows for an entry whose tags hold
// no type tag, one written before the rule (REQ-1292).
const NoType = "-"

// TagLesson is a free tag since REQ-0242 gave it no behaviour; the
// SessionStart of julieta handoff show still prints the entries that
// carry it until T172 replaces it with the index.
const TagLesson = "lesson"

// Type returns the type tag of tags, the first in TypeTags order when a
// planted file holds more than one, or NoType when they hold none.
func Type(tags []string) string {
	for _, t := range TypeTags {
		if slices.Contains(tags, t) {
			return t
		}
	}
	return NoType
}

// CheckType returns why tags do not hold exactly one type tag (03 3.9,
// REQ-1287), naming the five. Decode does not call it, so an entry
// file written before the rule is still read (REQ-1292); add and edit
// do, on the tags they would write.
func CheckType(tags []string) error {
	var held []string
	for _, t := range TypeTags {
		if slices.Contains(tags, t) {
			held = append(held, t)
		}
	}
	if len(held) == 1 {
		return nil
	}
	what := "no type tag"
	if len(held) > 1 {
		what = "the type tags " + strings.Join(held, " and ")
	}
	return fmt.Errorf("%w: the tags hold %s; an entry carries exactly one of %s", ErrInvalid, what, strings.Join(TypeTags, ", "))
}

// The bounds of the fields an agent supplies.
const (
	// maxTitle is the most bytes of a title.
	maxTitle = 200
	// maxTags is the most tags of an entry.
	maxTags = 32
)

// tagForm is the form of a tag: a lowercase word of at most 32 bytes.
var tagForm = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// schemaField reads a schema field of the form "memory-entry/v<n>".
var schemaField = regexp.MustCompile(`^memory-entry/v([1-9][0-9]{0,8})$`)

// Entry is a memory entry, the format memory-entry.v1 (03 3.9): its
// YAML front matter, whose fields are in the order of the file, and its
// markdown body. julieta stamps ID, CreatedAt and UpdatedAt (REQ-0656);
// the agent supplies the rest. Its JSON names are those of the schema
// schemas/memory-entry.v1.json, which describes the front matter.
type Entry struct {
	// Schema is "memory-entry/v1".
	Schema string `json:"schema" yaml:"schema"`
	// ID is the entry's lowercase ULID, also the name of its file.
	ID canon.ULID `json:"id" yaml:"id"`
	// Title is one line of text.
	Title string `json:"title" yaml:"title"`
	// Status is open or done.
	Status string `json:"status" yaml:"status"`
	// Tags are sorted and distinct; one of them is a type tag (Type).
	Tags []string `json:"tags" yaml:"tags,flow"`
	// CreatedAt is when julieta added the entry, in UTC, to the second.
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`
	// UpdatedAt is when julieta last changed it, in UTC, to the second.
	UpdatedAt time.Time `json:"updatedAt" yaml:"updatedAt"`
	// Body is the markdown after the front matter.
	Body string `json:"-" yaml:"-"`
}

// frontKeys are the keys of the front matter, every one required.
var frontKeys = []string{"schema", "id", "title", "status", "tags", "createdAt", "updatedAt"}

// Content is what the agent supplies of an entry: the input of the
// digest memory-content (03 3.12, REQ-0259), which the duplicate rule
// of julieta memory add compares (04 4.1, Idempotency).
type Content struct {
	Body   string   `json:"body"`
	Status string   `json:"status"`
	Tags   []string `json:"tags"`
	Title  string   `json:"title"`
}

// CanonKind declares the kind of Content, memory-content.
func (Content) CanonKind() canon.MemoryContentKind { return canon.MemoryContent }

// Content returns the content of e.
func (e Entry) Content() Content {
	return Content{Body: e.Body, Status: e.Status, Tags: e.Tags, Title: e.Title}
}

// Normal returns c with its tags sorted and distinct, an empty list for
// none, and the status open when it is empty: the form julieta stores
// and digests.
func (c Content) Normal() Content {
	tags := slices.Clone(c.Tags)
	slices.Sort(tags)
	c.Tags = slices.Compact(tags)
	if c.Tags == nil {
		c.Tags = []string{}
	}
	if c.Status == "" {
		c.Status = StatusOpen
	}
	return c
}

// Digest returns the digest memory-content of the normal form of c.
func (c Content) Digest() (canon.Sum[canon.MemoryContentKind], error) {
	return canon.Digest(canon.MemoryContent, c.Normal())
}

// Check returns why c is no content of an entry: a title that is empty,
// longer than 200 bytes or not one line of printable text, a status
// other than open and done, more than 32 tags or a tag not of the form
// [a-z0-9][a-z0-9-]{0,31}, or a body that is not UTF-8.
func (c Content) Check() error {
	switch {
	case c.Title == "" || strings.TrimSpace(c.Title) != c.Title:
		return fmt.Errorf("%w: the title is empty or starts or ends with a space", ErrInvalid)
	case len(c.Title) > maxTitle:
		return fmt.Errorf("%w: the title has %d bytes, over %d", ErrInvalid, len(c.Title), maxTitle)
	case !utf8.ValidString(c.Title) || strings.ContainsFunc(c.Title, func(r rune) bool { return !unicode.IsPrint(r) && r != ' ' }):
		return fmt.Errorf("%w: the title is not one line of printable text", ErrInvalid)
	case c.Status != StatusOpen && c.Status != StatusDone:
		return fmt.Errorf("%w: the status \"%s\" is not %s or %s", ErrInvalid, clip(c.Status), StatusOpen, StatusDone)
	case len(c.Tags) > maxTags:
		return fmt.Errorf("%w: %d tags, over %d", ErrInvalid, len(c.Tags), maxTags)
	case !utf8.ValidString(c.Body):
		return fmt.Errorf("%w: the body is not UTF-8", ErrInvalid)
	}
	for _, tag := range c.Tags {
		if !tagForm.MatchString(tag) {
			return fmt.Errorf("%w: the tag \"%s\" is not of the form [a-z0-9][a-z0-9-]{0,31}", ErrInvalid, clip(tag))
		}
	}
	return nil
}

// check returns why e is no entry of the format: its content fails
// Content.Check, its tags are null or not distinct, its id is zero, or
// its times are not UTC seconds with CreatedAt not after UpdatedAt.
// The tags may be in any order, as in the example of 03 3.9.
func (e Entry) check() error {
	c := e.Content()
	if err := c.Check(); err != nil {
		return err
	}
	if c.Tags == nil || len(c.Normal().Tags) != len(c.Tags) {
		return fmt.Errorf("%w: the tags are null or not distinct", ErrInvalid)
	}
	if e.ID == (canon.ULID{}) {
		return fmt.Errorf("%w: the id is zero", ErrInvalid)
	}
	for _, t := range []time.Time{e.CreatedAt, e.UpdatedAt} {
		if t.Location() != time.UTC || !t.Truncate(time.Second).Equal(t) {
			return fmt.Errorf("%w: the time %s is not UTC to the second", ErrInvalid, t)
		}
	}
	if e.CreatedAt.After(e.UpdatedAt) {
		return fmt.Errorf("%w: createdAt is after updatedAt", ErrInvalid)
	}
	return nil
}

// Stamp returns the form of t julieta writes: UTC, to the second.
func Stamp(t time.Time) time.Time { return t.UTC().Truncate(time.Second) }

// The lines that open and close the front matter.
const (
	frontOpen  = "---\n"
	frontClose = "\n---\n"
)

// Encode returns the file of e: the front matter between two "---"
// lines, with the tags sorted, then the body as it is. An entry that
// breaks a rule of 03 3.9, or whose file passes MaxFileSize, is an
// error.
func Encode(e Entry) ([]byte, error) {
	e.Tags = slices.Clone(e.Tags) // nil stays nil, which check refuses
	slices.Sort(e.Tags)
	if e.Schema != Field {
		return nil, fmt.Errorf("%w: the schema \"%s\" is not %s", ErrInvalid, clip(e.Schema), Field)
	}
	if err := e.check(); err != nil {
		return nil, err
	}
	front, err := yaml.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("memstore: encode: %w", err)
	}
	var b bytes.Buffer
	b.WriteString(frontOpen)
	b.Write(front)
	b.WriteString(frontClose[1:])
	b.WriteString(e.Body)
	if b.Len() > MaxFileSize {
		return nil, fmt.Errorf("%w: the file has %d bytes, over %d", ErrInvalid, b.Len(), MaxFileSize)
	}
	return b.Bytes(), nil
}

// Decode reads the file of an entry strictly (03, opening, REQ-0168):
// at most MaxFileSize bytes of UTF-8; a front matter between a first
// line "---" and the next, of one YAML mapping with no anchor, alias,
// merge key, tag outside YAML's core schema (an include among them) or
// duplicate key; then go.yaml.in/yaml/v3 into Entry, every key of v1
// required and a key this build does not know skipped, as a newer
// writer of the same version may add one (REQ-0163); then the rules of
// 03 3.9, with the tags sorted on the way in. The schema field is read
// from the parsed mapping before the keys are judged, so a newer
// version is format-newer (RJ-311) though it holds keys v1 does not
// know; a front matter that does not parse, or that the subset grammar
// refuses, is format-newer when the text of its schema line names a
// newer version (checkNewer), so a v2 file with an anchor or a syntax
// error is too. Every other refusal wraps ErrInvalid.
func Decode(data []byte) (Entry, error) {
	if len(data) > MaxFileSize {
		return Entry{}, fmt.Errorf("%w: the file has %d bytes, over %d", ErrInvalid, len(data), MaxFileSize)
	}
	if !utf8.Valid(data) {
		return Entry{}, fmt.Errorf("%w: the file is not UTF-8", ErrInvalid)
	}
	rest, ok := bytes.CutPrefix(data, []byte(frontOpen))
	if !ok {
		return Entry{}, fmt.Errorf("%w: the file does not start with a line ---", ErrInvalid)
	}
	var front, body []byte
	if b, ok := bytes.CutPrefix(rest, []byte(frontClose[1:])); ok {
		body = b
	} else {
		i := bytes.Index(rest, []byte(frontClose))
		if i < 0 {
			return Entry{}, fmt.Errorf("%w: the front matter has no closing line ---", ErrInvalid)
		}
		front, body = rest[:i+1], rest[i+len(frontClose):]
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(front, &doc); err != nil {
		return Entry{}, refused(front, err)
	}
	if err := checkNode(&doc); err != nil {
		return Entry{}, refused(front, err)
	}
	if err := checkVersion(doc.Content[0]); err != nil {
		return Entry{}, err
	}
	if err := checkKeys(doc.Content[0]); err != nil {
		return Entry{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	var e Entry
	dec := yaml.NewDecoder(bytes.NewReader(front))
	if err := dec.Decode(&e); err != nil {
		return Entry{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := dec.Decode(new(yaml.Node)); !errors.Is(err, io.EOF) {
		return Entry{}, fmt.Errorf("%w: the front matter holds more than one document", ErrInvalid)
	}
	e.Body = string(body)
	if err := e.check(); err != nil {
		return Entry{}, err
	}
	slices.Sort(e.Tags)
	return e, nil
}

// schemaLine reads a top-level schema line of a front matter, before
// any YAML parse: the key at the start of a line, the value plain or
// quoted, and an optional comment.
var schemaLine = regexp.MustCompile(`(?m)^schema:[ \t]+["']?memory-entry/v([1-9][0-9]{0,8})["']?[ \t]*(?:#.*)?$`)

// refused returns the error of a front matter that does not parse or
// that the subset grammar refuses for the reason err: format-newer when
// its schema line names a newer version (checkNewer), ErrInvalid
// otherwise.
func refused(front []byte, err error) error {
	if newer := checkNewer(front); newer != nil {
		return newer
	}
	return fmt.Errorf("%w: %w", ErrInvalid, err)
}

// checkNewer returns format-newer (RJ-311) when the first schema line
// of the front matter text names a version newer than Version, and nil
// otherwise. It reads text, so a line inside a multi-line scalar can
// look like a schema line: Decode calls it only for a front matter that
// does not parse or that the subset grammar refuses, where the parsed
// mapping cannot be read, and the file is refused either way.
func checkNewer(front []byte) error {
	sub := schemaLine.FindSubmatch(front)
	if sub == nil {
		return nil
	}
	if n, err := strconv.Atoi(string(sub[1])); err == nil && n > Version {
		return manifest.FormatNewer.New(fmt.Sprintf("the memory entry is memory-entry/v%d, newer than %s, the version this build reads", n, Field))
	}
	return nil
}

// coreTags are the tags of YAML's core schema that a front matter may
// resolve to.
var coreTags = []string{"!!str", "!!int", "!!bool", "!!null", "!!float", "!!timestamp", "!!seq", "!!map"}

// checkNode returns why doc, a decoded front matter, is outside the
// subset grammar: it is not one mapping, or a node in it has an anchor,
// is an alias, is a merge key or has a tag outside coreTags.
func checkNode(doc *yaml.Node) error {
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return errors.New("the front matter is not one mapping")
	}
	return checkTree(doc.Content[0], 0)
}

// checkKeys returns why the mapping m has a key twice or lacks a key of
// frontKeys; a key not in frontKeys is skipped.
func checkKeys(m *yaml.Node) error {
	seen := map[string]bool{}
	for i := 0; i < len(m.Content); i += 2 {
		key := m.Content[i].Value
		if seen[key] {
			return fmt.Errorf("the key \"%s\" is written twice", clip(key))
		}
		seen[key] = true
	}
	for _, key := range frontKeys {
		if !seen[key] {
			return fmt.Errorf("the key %s is missing", key)
		}
	}
	return nil
}

// maxDepth bounds the nesting of a front matter, whose fields nest two
// levels.
const maxDepth = 4

// checkTree returns why the node n or one below it is outside the
// subset grammar: an anchor, a merge key, a tag outside coreTags, a
// key written twice in one mapping at any depth, or a nesting deeper
// than maxDepth. An alias needs an anchor before it,
// which is refused first, and the parser refuses an alias to an anchor
// it has not seen, so no alias reaches past this check.
func checkTree(n *yaml.Node, depth int) error {
	switch {
	case depth > maxDepth:
		return errors.New("the front matter nests too deep")
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
			k := n.Content[i].Value
			if seen[k] {
				return fmt.Errorf("the key \"%s\" is written twice", clip(k))
			}
			seen[k] = true
		}
	}
	for _, c := range n.Content {
		if err := checkTree(c, depth+1); err != nil {
			return err
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
		v := m.Content[i+1].Value
		sub := schemaField.FindStringSubmatch(v)
		if sub == nil || m.Content[i+1].Kind != yaml.ScalarNode {
			return fmt.Errorf("%w: the schema \"%s\" is not memory-entry/v<n>", ErrInvalid, clip(v))
		}
		if n, _ := strconv.Atoi(sub[1]); n > Version {
			return manifest.FormatNewer.New(fmt.Sprintf("the memory entry is %s, newer than %s, the version this build reads", v, Field))
		}
		return nil
	}
	return fmt.Errorf("%w: no schema field", ErrInvalid)
}

// clipLen is the most bytes of an input that an error message quotes.
const clipLen = 64

// clip returns s cut to clipLen bytes and escaped with termsafe, so
// that an error about a long input stays short and a name an agent
// wrote cannot forge a line of the output. A message quotes it with
// "%s", never %q, which would escape the escapes again.
func clip(s string) string {
	if len(s) > clipLen {
		s = s[:clipLen] + "..."
	}
	return termsafe.Escape(s)
}
