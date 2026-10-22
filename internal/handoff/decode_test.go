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
