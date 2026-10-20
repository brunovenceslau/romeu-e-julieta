// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memstore_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// fixture returns the file testdata/name.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return data
}

// example is the entry of valid.md, the example of 03 3.9.
func example(t *testing.T) memstore.Entry {
	t.Helper()
	u, err := canon.ParseULID("01j9z3k8m6t2q4w5e7r9y1v3k5")
	require.NoError(t, err)
	at := time.Date(2026, 10, 2, 9, 20, 0, 0, time.UTC)
	return memstore.Entry{Schema: memstore.Field, ID: u, Title: "Flaky integration test on arm64", Status: memstore.StatusOpen,
		Tags: []string{"arm64", "ci"}, CreatedAt: at, UpdatedAt: at, Body: "Free-form markdown body.\n"}
}

// TestDecodeExample reads the example of 03 3.9, byte for byte, whose
// tags are not sorted and whose lines carry comments: the decoder sorts
// the tags, and Encode writes the entry in julieta's one spelling,
// which decodes to the same entry.
func TestDecodeExample(t *testing.T) {
	e, err := memstore.Decode(fixture(t, "valid.md"))
	require.NoError(t, err)
	assert.Equal(t, example(t), e)
	enc, err := memstore.Encode(e)
	require.NoError(t, err)
	assert.Equal(t, "---\nschema: memory-entry/v1\nid: 01j9z3k8m6t2q4w5e7r9y1v3k5\ntitle: Flaky integration test on arm64\n"+
		"status: open\ntags: [arm64, ci]\ncreatedAt: 2026-10-02T09:20:00Z\nupdatedAt: 2026-10-02T09:20:00Z\n---\n"+
		"Free-form markdown body.\n", string(enc))
	again, err := memstore.Decode(enc)
	require.NoError(t, err)
	assert.Equal(t, e, again)
}

// TestEncodeSortsTags writes the tags of an entry sorted, whatever
// their order, and leaves the entry it was given as it was.
func TestEncodeSortsTags(t *testing.T) {
	e := example(t)
	e.Tags = []string{"ci", "arm64"}
	enc, err := memstore.Encode(e)
	require.NoError(t, err)
	assert.Contains(t, string(enc), "\ntags: [arm64, ci]\n")
	assert.Equal(t, []string{"ci", "arm64"}, e.Tags)
}

// TestDecodeRefusesTheSubsetGrammar refuses, one fixture each, a front
// matter with an anchor, an alias, a merge key, an include, a
// duplicate key and an unknown key (03, opening, REQ-0168; 10 10.1).
func TestDecodeRefusesTheSubsetGrammar(t *testing.T) {
	for name, why := range map[string]string{
		"invalid-anchor.md":        "anchor",
		"invalid-alias.md":         "anchor",
		"invalid-merge-key.md":     "merge key",
		"invalid-include.md":       `the tag "!include"`,
		"invalid-duplicate-key.md": `the key "status" is written twice`,
		"invalid-unknown-key.md":   `the key "agent" is not a field`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := memstore.Decode(fixture(t, name))
			require.ErrorIs(t, err, memstore.ErrInvalid)
			assert.Contains(t, err.Error(), why)
		})
	}
}

// TestDecodeRefusesAnAliasAlone refuses an alias whose anchor is
// elsewhere: the check names the alias when it meets it first.
func TestDecodeRefusesAnAliasAlone(t *testing.T) {
	data := strings.Replace(string(fixture(t, "valid.md")), "tags: [ci, arm64]", "tags: *x", 1)
	_, err := memstore.Decode([]byte(data))
	require.ErrorIs(t, err, memstore.ErrInvalid)
}

// TestDecodeRefusesANewerVersion refuses a memory-entry/v2 with
// format-newer (RJ-311), though it holds a field v1 does not know.
func TestDecodeRefusesANewerVersion(t *testing.T) {
	_, err := memstore.Decode(fixture(t, "newer.md"))
	e, ok := errors.AsType[*cli.Error](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, "RJ-311", e.ID)
	assert.Equal(t, 2, e.Exit)
}

// TestDecodeNewerFirst reports format-newer (RJ-311) for a v2 file
// whatever its front matter holds: an anchor, a merge key, an unknown
// key, or text that is no YAML at all (03, opening).
func TestDecodeNewerFirst(t *testing.T) {
	for name, front := range map[string]string{
		"an anchor":      "schema: memory-entry/v2\ntags: &t [a]\n",
		"a merge key":    "schema: memory-entry/v2\n<<: {a: 1}\n",
		"a syntax error": "schema: memory-entry/v2\ntitle: [unclosed\n",
		"quoted":         "schema: \"memory-entry/v3\"   # a comment\nx: {\n",
		"a later line":   "id: x\nschema: memory-entry/v2\n: :\n",
	} {
		_, err := memstore.Decode([]byte("---\n" + front + "---\nbody\n"))
		e, ok := errors.AsType[*cli.Error](err)
		require.True(t, ok, "%s: %v", name, err)
		assert.Equal(t, "RJ-311", e.ID, name)
	}
}

// TestDecodeRefusesOverTheCap refuses a file over the file cap, 1 MiB
// (08 8.6), before it parses it.
func TestDecodeRefusesOverTheCap(t *testing.T) {
	data := append(fixture(t, "valid.md"), bytes.Repeat([]byte("x"), memstore.MaxFileSize)...)
	_, err := memstore.Decode(data)
	require.ErrorIs(t, err, memstore.ErrInvalid)
	assert.Contains(t, err.Error(), "over 1048576")
}

// TestDecodeRefuses refuses each other way a file breaks 03 3.9.
func TestDecodeRefuses(t *testing.T) {
	valid := string(fixture(t, "valid.md"))
	for name, data := range map[string]string{
		"no front matter":      "Free-form\n",
		"no closing line":      "---\nschema: memory-entry/v1\n",
		"not UTF-8":            valid + "\xff",
		"not a mapping":        "---\n- a\n---\n",
		"a missing key":        strings.Replace(valid, "status: open            # open | done\n", "", 1),
		"another format":       strings.Replace(valid, "memory-entry/v1", "handoff/v1", 1),
		"no schema":            strings.Replace(valid, "schema: memory-entry/v1\n", "", 1),
		"an older version":     strings.Replace(valid, "memory-entry/v1", "memory-entry/v0", 1),
		"a wrong type":         strings.Replace(valid, "tags: [ci, arm64]", "tags: ci", 1),
		"an uppercase id":      strings.Replace(valid, "01j9z3k8m6t2q4w5e7r9y1v3k5", "01J9Z3K8M6T2Q4W5E7R9Y1V3K5", 1),
		"a zero id":            strings.Replace(valid, "01j9z3k8m6t2q4w5e7r9y1v3k5", "00000000000000000000000000", 1),
		"another status":       strings.Replace(valid, "status: open", "status: closed", 1),
		"an empty title":       strings.Replace(valid, "title: Flaky integration test on arm64", "title: ''", 1),
		"a long title":         strings.Replace(valid, "title: Flaky integration test on arm64", "title: "+strings.Repeat("t", 201), 1),
		"a control in a title": strings.Replace(valid, "title: Flaky integration test on arm64", `title: "a\tb"`, 1),
		"a bad tag":            strings.Replace(valid, "[ci, arm64]", "[ARM]", 1),
		"duplicate tags":       strings.Replace(valid, "[ci, arm64]", "[ci, ci]", 1),
		"null tags":            strings.Replace(valid, "[ci, arm64]", "null", 1),
		"too many tags":        strings.Replace(valid, "[ci, arm64]", "["+strings.Repeat("a,", 33)+"a]", 1),
		"an offset time":       strings.Replace(valid, "createdAt: 2026-10-02T09:20:00Z", "createdAt: 2026-10-02T09:20:00+02:00", 1),
		"a fraction":           strings.Replace(valid, "createdAt: 2026-10-02T09:20:00Z", "createdAt: 2026-10-02T09:20:00.5Z", 1),
		"created after":        strings.Replace(valid, "createdAt: 2026-10-02T09:20:00Z", "createdAt: 2026-10-03T09:20:00Z", 1),
		"two documents":        strings.Replace(valid, "updatedAt: 2026-10-02T09:20:00Z\n", "updatedAt: 2026-10-02T09:20:00Z\n...\n--- x\n", 1),
		"too deep":             strings.Replace(valid, "[ci, arm64]", "[[[[[[a]]]]]]", 1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := memstore.Decode([]byte(data))
			require.ErrorIs(t, err, memstore.ErrInvalid)
		})
	}
}

// TestDecodeAnEmptyFrontMatter refuses a file whose front matter is
// empty: every key is required.
func TestDecodeAnEmptyFrontMatter(t *testing.T) {
	_, err := memstore.Decode([]byte("---\n---\nbody\n"))
	require.ErrorIs(t, err, memstore.ErrInvalid)
}

// TestEncodeRefuses refuses an entry that breaks a rule of 03 3.9.
func TestEncodeRefuses(t *testing.T) {
	for name, edit := range map[string]func(*memstore.Entry){
		"another schema": func(e *memstore.Entry) { e.Schema = "memory-entry/v2" },
		"no id":          func(e *memstore.Entry) { e.ID = canon.ULID{} },
		"a local time":   func(e *memstore.Entry) { e.CreatedAt = e.CreatedAt.In(time.FixedZone("x", 3600)) },
		"no tags":        func(e *memstore.Entry) { e.Tags = nil },
		"a body over":    func(e *memstore.Entry) { e.Body = strings.Repeat("x", memstore.MaxFileSize) },
		"a bad body":     func(e *memstore.Entry) { e.Body = "\xff" },
	} {
		t.Run(name, func(t *testing.T) {
			e := example(t)
			edit(&e)
			_, err := memstore.Encode(e)
			require.ErrorIs(t, err, memstore.ErrInvalid)
		})
	}
}

// TestContentDigest digests the normal form of the content: the order
// and repeats of the tags and an empty status do not change it, and
// the body does.
func TestContentDigest(t *testing.T) {
	a, err := memstore.Content{Title: "t", Body: "b", Tags: []string{"y", "x", "x"}}.Digest()
	require.NoError(t, err)
	b, err := memstore.Content{Title: "t", Body: "b", Tags: []string{"x", "y"}, Status: memstore.StatusOpen}.Digest()
	require.NoError(t, err)
	assert.Equal(t, a, b)
	c, err := memstore.Content{Title: "t", Body: "c", Tags: []string{"x", "y"}}.Digest()
	require.NoError(t, err)
	assert.NotEqual(t, a, c)
	n := memstore.Content{Title: "t"}.Normal()
	assert.Equal(t, []string{}, n.Tags)
	assert.Equal(t, memstore.StatusOpen, n.Status)
}

// TestTempTime reads the time of a temporary name and refuses any other
// name (REQ-0654).
func TestTempTime(t *testing.T) {
	at := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	u, err := canon.NewULID(at, canon.NewULIDSource(bytes.NewReader(make([]byte, 10))))
	require.NoError(t, err)
	got, ok := memstore.TempTime(memstore.TempName(u))
	require.True(t, ok)
	assert.Equal(t, at, got)
	for _, name := range []string{".tmp-", ".tmp-x", "tmp-" + u.String(), ".tmp-" + strings.ToUpper(u.String()), u.String()} {
		assert.False(t, memstore.IsTemp(name), name)
	}
}
