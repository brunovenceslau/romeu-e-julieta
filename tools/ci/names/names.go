// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package names holds the denylist of forbidden names and the one
// definition of a match (10 10.2, "Forbidden names"). Every check that
// looks for a forbidden name calls Match, so the hook, hygiene and the
// pull request check cannot disagree about what a hit is.
//
// The file stores no name. An entry is a list of segments, and a
// segment is a length and the sha256 of a lowercased piece of the name.
// The hash keeps the plaintext out of a search of the tree; it is not a
// secret, because a short segment is recovered by trying each string of
// its length.
package names

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Path is where the denylist lives, relative to the repository root.
const Path = "tools/ci/denylist.yaml"

// header opens the file Marshal writes, so the denylist carries its own
// license line however it was produced.
const header = `# SPDX-FileCopyrightText: 2026 Bruno Venceslau
# SPDX-License-Identifier: GPL-3.0-only

# Names this repository must not contain, hashed. Add an entry with
# "go run ./tools/ci hygiene add", on the host, in a terminal.
`

// segment is one piece of a name: its length in bytes and the sha256 of
// its lowercased text.
type segment struct {
	length int
	sum    [sha256.Size]byte
}

// Entry is one forbidden name, as an ordered list of segments.
type Entry struct {
	segments []segment
}

// List is a denylist. The zero value is an empty list.
type List struct {
	entries []Entry
}

// The three types below are the file format.
type fileList struct {
	Entries []fileEntry `yaml:"entries"`
}

type fileEntry struct {
	Segments []fileSegment `yaml:"segments"`
}

type fileSegment struct {
	Length int    `yaml:"length"`
	SHA256 string `yaml:"sha256"`
}

// Load reads the denylist at path. A missing file is an error that
// wraps fs.ErrNotExist; a file without entries loads as an empty list,
// and the caller decides what that means.
func Load(path string) (*List, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	l, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return l, nil
}

// Parse decodes a denylist file. It refuses a key it does not know and
// a segment that could never match, so a typing error does not load as
// an entry that silently matches nothing.
func Parse(data []byte) (*List, error) {
	var f fileList
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decode denylist: %w", err)
	}
	l := &List{}
	for i, fe := range f.Entries {
		if len(fe.Segments) == 0 {
			return nil, fmt.Errorf("denylist entry %d has no segment", i+1)
		}
		var e Entry
		for j, fs := range fe.Segments {
			sum, err := hex.DecodeString(fs.SHA256)
			if err != nil || len(sum) != sha256.Size {
				return nil, fmt.Errorf("denylist entry %d, segment %d: sha256 is not 64 hex digits", i+1, j+1)
			}
			if fs.Length <= 0 {
				return nil, fmt.Errorf("denylist entry %d, segment %d: length is not positive", i+1, j+1)
			}
			e.segments = append(e.segments, segment{length: fs.Length, sum: [sha256.Size]byte(sum)})
		}
		l.entries = append(l.entries, e)
	}
	return l, nil
}

// Marshal encodes the list in the file format, header included.
func (l *List) Marshal() []byte {
	f := fileList{Entries: []fileEntry{}}
	for _, e := range l.entries {
		fe := fileEntry{}
		for _, s := range e.segments {
			fe.Segments = append(fe.Segments, fileSegment{Length: s.length, SHA256: hex.EncodeToString(s.sum[:])})
		}
		f.Entries = append(f.Entries, fe)
	}
	var buf bytes.Buffer
	buf.WriteString(header)
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	// Encoding plain structs of ints and hex strings into a buffer has
	// no failing input.
	if err := enc.Encode(f); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// Len reports the number of entries.
func (l *List) Len() int { return len(l.entries) }

// Add appends e and reports whether it was new.
func (l *List) Add(e Entry) bool {
	for _, have := range l.entries {
		if have.equal(e) {
			return false
		}
	}
	l.entries = append(l.entries, e)
	return true
}

// Merge adds each entry of o that l does not hold yet.
func (l *List) Merge(o *List) {
	for _, e := range o.entries {
		l.Add(e)
	}
}

func (e Entry) equal(o Entry) bool {
	if len(e.segments) != len(o.segments) {
		return false
	}
	for i := range e.segments {
		if e.segments[i] != o.segments[i] {
			return false
		}
	}
	return true
}

// Segments reports how many segments the entry has.
func (e Entry) Segments() int { return len(e.segments) }

// NewEntry builds the entry for a name typed as one line with a space
// between segments. It lowercases the name, refuses a segment that
// holds anything but ASCII letters and digits, and proves that the
// matcher finds the name before it returns: an entry that is wrong
// matches nothing, and no test could see that, because the name is not
// in the tree. Its errors never repeat the name.
func NewEntry(name string) (Entry, error) {
	pieces := strings.Split(strings.Trim(name, " "), " ")
	var e Entry
	var together []byte
	for _, p := range pieces {
		if p == "" {
			if len(pieces) == 1 {
				return Entry{}, errors.New("the name is empty")
			}
			continue // more than one space between two segments
		}
		low := lower([]byte(p))
		for _, c := range low {
			if !isLetterOrDigit(c) {
				return Entry{}, errors.New("a segment holds a character that is not an ASCII letter or digit")
			}
		}
		e.segments = append(e.segments, segment{length: len(low), sum: sha256.Sum256(low)})
		together = append(together, low...)
	}
	probe := &List{entries: []Entry{e}}
	if !probe.Match(together) {
		return Entry{}, errors.New("the matcher does not find the name it was given; nothing was written")
	}
	return e, nil
}

// Match reports whether line contains a denylist entry: with ASCII
// letters lowercased, the entry's segments in order, with nothing
// between two of them but characters that are neither ASCII letters,
// ASCII digits nor a slash.
//
// A slash is left out on purpose: two halves joined by one are the
// shape of a registry or repository path of another product.
func (l *List) Match(line []byte) bool {
	if len(l.entries) == 0 {
		return false
	}
	low := lower(line)
	for _, e := range l.entries {
		for start := 0; start+e.segments[0].length <= len(low); start++ {
			if e.matchAt(low, start) {
				return true
			}
		}
	}
	return false
}

// matchAt reports whether the entry's segments are found in order from
// start. A segment begins with a letter or a digit, so skipping every
// separator before it loses no match.
func (e Entry) matchAt(low []byte, start int) bool {
	pos := start
	for i, s := range e.segments {
		if i > 0 {
			for pos < len(low) && isSeparator(low[pos]) {
				pos++
			}
		}
		if pos+s.length > len(low) || sha256.Sum256(low[pos:pos+s.length]) != s.sum {
			return false
		}
		pos += s.length
	}
	return true
}

// MatchLines returns the 1-based numbers of the lines of content that
// match. Text is matched one line at a time, so a name split across two
// lines is not found; that limit is stated in 10 10.2.
func (l *List) MatchLines(content []byte) []int {
	if len(l.entries) == 0 {
		return nil
	}
	var hits []int
	for n := 1; len(content) > 0; n++ {
		line, rest, _ := bytes.Cut(content, []byte{'\n'})
		if l.Match(line) {
			hits = append(hits, n)
		}
		content = rest
	}
	return hits
}

func lower(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return out
}

func isLetterOrDigit(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func isSeparator(c byte) bool {
	return !isLetterOrDigit(c) && c != '/'
}
