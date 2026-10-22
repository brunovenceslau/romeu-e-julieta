// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/handoff"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// fixture returns the bytes of testdata/name.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return data
}

// TestDecodeValid reads the example of 03 3.10 for each kind, and
// encodes it back to the same bytes.
func TestDecodeValid(t *testing.T) {
	for name, kind := range map[string]handoff.Kind{"valid-clear.md": handoff.Clear, "valid-final.md": handoff.Final, "valid-facts.md": handoff.Facts} {
		t.Run(name, func(t *testing.T) {
			data := fixture(t, name)
			f, err := handoff.Decode(data)
			require.NoError(t, err)
			assert.Equal(t, kind, f.Kind)
			assert.Equal(t, time.Date(2026, 10, 2, 18, 30, 0, 0, time.UTC), f.CreatedAt)
			assert.Equal(t, []handoff.RepoFacts{{Dir: "shop-api", Branch: "feat/cart", Head: "0123456789abcdef0123456789abcdef01234567",
				Upstream: "origin/feat/cart", Ahead: 2, Dirty: 3}}, f.Repos)
			enc, err := handoff.Encode(f)
			require.NoError(t, err)
			assert.Equal(t, string(data), string(enc))
		})
	}
}

// TestDecodeRefuses reads one fixture for each construct the strict
// subset grammar refuses (REQ-0168): an anchor, an alias, a merge key,
// an include and a duplicate key; and the bodies that break REQ-0245.
func TestDecodeRefuses(t *testing.T) {
	for name, why := range map[string]string{
		"invalid-anchor.md":              "anchor",
		"invalid-alias.md":               "anchor",
		"invalid-merge-key.md":           "merge key",
		"invalid-include.md":             "tag",
		"invalid-duplicate-key.md":       "written twice",
		"invalid-missing-headings.md":    `the heading "## Decisions"`,
		"invalid-final-five-headings.md": `the heading "## What would be lost"`,
		"invalid-facts-body.md":          "no body",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := handoff.Decode(fixture(t, name))
			require.ErrorIs(t, err, handoff.ErrInvalid)
			assert.Contains(t, err.Error(), why)
		})
	}
}

// TestDecodeRefusesAMissingKey refuses a file that lacks one key of v1,
// in the front matter or in a repo: skipping an unknown key does not
// relax the requirement of the known ones (03, opening, REQ-0163).
func TestDecodeRefusesAMissingKey(t *testing.T) {
	valid := string(fixture(t, "valid-clear.md"))
	for name, doc := range map[string]string{
		"a front matter key": strings.Replace(valid, "project: shop\n", "", 1),
		"a repo key":         strings.Replace(valid, "    stashes: 0\n", "", 1),
	} {
		t.Run(name, func(t *testing.T) {
			require.NotEqual(t, valid, doc, "the fixture no longer holds the line this case removes")
			_, err := handoff.Decode([]byte(doc))
			require.ErrorIs(t, err, handoff.ErrInvalid)
			assert.Contains(t, err.Error(), "is missing from")
		})
	}
}

// TestDecodeIgnoresAnUnknownKey reads a handoff file that a newer writer
// gave a key, in the front matter and in a repo, as the same file
// without it (03, opening, REQ-0163).
func TestDecodeIgnoresAnUnknownKey(t *testing.T) {
	want, err := handoff.Decode(fixture(t, "valid-clear.md"))
	require.NoError(t, err)
	for _, name := range []string{"unknown-key.md", "unknown-repo-key.md"} {
		t.Run(name, func(t *testing.T) {
			got, err := handoff.Decode(fixture(t, name))
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

// TestDecodeHostileUnknownKeys holds what a key this build skips may not
// do, whoever wrote it (03, opening, REQ-0163): change what a known key
// says (a key of another case is another key, in either order), pass a
// duplicate, a nest past the depth cap or a file over the size cap
// unseen, or put its own text in an error.
func TestDecodeHostileUnknownKeys(t *testing.T) {
	valid := string(fixture(t, "valid-clear.md"))
	want, err := handoff.Decode([]byte(valid))
	require.NoError(t, err)
	// Keys go in the front matter, before and after the known ones.
	after := func(keys string) []byte {
		return []byte(strings.Replace(valid, "agent: sbx-kit-claude\n", "agent: sbx-kit-claude\n"+keys, 1))
	}
	before := func(keys string) []byte {
		return []byte(strings.Replace(valid, "schema: handoff/v1\n", "schema: handoff/v1\n"+keys, 1))
	}
	for name, keys := range map[string]string{
		"a case variant of a known key":  "Project: x\nKIND: final\nSchema: y\n",
		"an escape in a key and a value": "\"\\e[31mred\": \"\\a\\e[0m\"\n",
	} {
		for pos, build := range map[string]func(string) []byte{"after": after, "before": before} {
			got, err := handoff.Decode(build(keys))
			require.NoError(t, err, name+" "+pos)
			assert.Equal(t, want, got, name+" "+pos)
		}
	}
	for name, keys := range map[string]string{
		"a duplicate in an unknown key":          "x:\n  a: 1\n  a: 2\n",
		"a nest past the depth cap":              "x: " + strings.Repeat("[", 12) + strings.Repeat("]", 12) + "\n",
		"a duplicate known key beside an escape": "\"\\e[31mred\": 1\nproject: twice\n",
		"the excess over the file cap":           "x: " + strings.Repeat("y", memstore.MaxFileSize) + "\n",
	} {
		_, err := handoff.Decode(after(keys))
		require.ErrorIs(t, err, handoff.ErrInvalid, name)
		assert.NotContains(t, err.Error(), "\x1b", name)
		assert.NotContains(t, err.Error(), "red", name)
	}
	// The name of a duplicate is in the error, escaped by termsafe: no raw
	// control character reaches the output.
	_, err = handoff.Decode(after("\"\\e[31mred\": 1\n\"\\e[31mred\": 2\n"))
	require.ErrorIs(t, err, handoff.ErrInvalid)
	assert.NotContains(t, err.Error(), "\x1b")
}

// TestDecodeRefusesAFileOverTheCap refuses a file over the file cap of
// 08 8.6 before it parses it.
func TestDecodeRefusesAFileOverTheCap(t *testing.T) {
	data := append(fixture(t, "valid-clear.md"), bytes.Repeat([]byte("x"), memstore.MaxFileSize)...)
	_, err := handoff.Decode(data)
	require.ErrorIs(t, err, handoff.ErrInvalid)
	assert.Contains(t, err.Error(), "over")
}

// TestDecodeRefusesANewerVersion refuses handoff/v2 with format-newer
// (RJ-311, REQ-0163), also when the file holds keys v1 does not know or
// an anchor.
func TestDecodeRefusesANewerVersion(t *testing.T) {
	for _, name := range []string{"newer.md", "newer-anchor.md"} {
		_, err := handoff.Decode(fixture(t, name))
		require.Error(t, err, name)
		assert.Equal(t, "RJ-311", cli.Of(err).ID, name)
	}
}

// TestCheckHeadings needs the headings in order, as lines.
func TestCheckHeadings(t *testing.T) {
	full := "## Goal\n## State\n## Decisions\n## Next steps\n## Open questions\n"
	require.NoError(t, handoff.CheckHeadings(handoff.Clear, full))
	require.NoError(t, handoff.CheckHeadings(handoff.Clear, "x\r\n"+strings.ReplaceAll(full, "\n", "  \r\n")))
	require.Error(t, handoff.CheckHeadings(handoff.Final, full), "final needs a sixth")
	require.NoError(t, handoff.CheckHeadings(handoff.Final, full+"## What would be lost\n"))
	require.Error(t, handoff.CheckHeadings(handoff.Clear, "## State\n## Goal\n## Decisions\n## Next steps\n## Open questions\n"), "out of order")
	require.Error(t, handoff.CheckHeadings(handoff.Clear, "text ## Goal\n"+full[len("## Goal\n"):]), "a heading is a line")
	require.Error(t, handoff.CheckHeadings(handoff.Facts, ""))
}

// TestDecodeRefusesANewerVersionInFlowStyle: a v2 file in YAML flow
// style, with an anchor or a syntax error, is format-newer too.
func TestDecodeRefusesANewerVersionInFlowStyle(t *testing.T) {
	for _, front := range []string{"{schema: handoff/v2, kind: clear, x: &a 1}", "{schema: \"handoff/v2\", kind: [}"} {
		_, err := handoff.Decode([]byte("---\n" + front + "\n---\n"))
		require.Error(t, err, front)
		assert.Equal(t, "RJ-311", cli.Of(err).ID, front)
	}
}
