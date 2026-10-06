// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package names

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// madeUp is the name every test here uses. It is an invented word pair
// and names nothing.
const madeUp = "zorvex quimby"

func listOf(t *testing.T, madeUpNames ...string) *List {
	t.Helper()
	l := &List{}
	for _, n := range madeUpNames {
		e, err := NewEntry(n)
		require.NoError(t, err, "NewEntry")
		l.Add(e)
	}
	return l
}

// TestMatcherSelfTest plants a made-up name against its own denylist:
// written together, with each separator, and with a slash. Each form
// except the slash must match (10 10.2, "Forbidden names").
func TestMatcherSelfTest(t *testing.T) {
	l := listOf(t, madeUp)
	tests := []struct {
		form string
		line string
		want bool
	}{
		{"together", "see zorvexquimby here", true},
		{"hyphen", "see zorvex-quimby here", true},
		{"underscore", "see zorvex_quimby here", true},
		{"dot", "see zorvex.quimby here", true},
		{"space", "see zorvex quimby here", true},
		{"several spaces", "see zorvex   quimby here", true},
		{"mixed separators", "see zorvex._- quimby here", true},
		{"slash", "see zorvex/quimby here", false},
		{"slash among separators", "see zorvex-/-quimby here", false},
	}
	for _, tt := range tests {
		t.Run(tt.form, func(t *testing.T) {
			assert.Equal(t, tt.want, l.Match([]byte(tt.line)), "Match(%q)", tt.line)
		})
	}
}

func TestMatch(t *testing.T) {
	l := listOf(t, madeUp, "plimsor")
	tests := []struct {
		name string
		line string
		want bool
	}{
		{"empty line", "", false},
		{"unrelated", "nothing to see", false},
		{"upper case", "ZORVEX-Quimby", true},
		{"inside a longer word", "prezorvexquimbypost", true},
		{"inside a path", "a/b/zorvex-quimby/c.go", true},
		{"one segment as a substring", "xxplimsorxx", true},
		{"one segment in upper case", "PLIMSOR", true},
		{"first half alone", "zorvex", false},
		{"second half alone", "quimby", false},
		{"halves in the wrong order", "quimby zorvex", false},
		{"a letter between the halves", "zorvex a quimby", false},
		{"a digit between the halves", "zorvex 1 quimby", false},
		{"a non-ASCII separator", "zorvex\u00b7quimby", true},
		{"a tab", "zorvex\tquimby", true},
		{"a failed start, then a match", "zorvex x zorvex quimby", true},
		{"at the end of the line", "the zorvex quimby", true},
		{"cut short", "zorvex quimb", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, l.Match([]byte(tt.line)), "Match(%q)", tt.line)
		})
	}
}

func TestMatchLines(t *testing.T) {
	l := listOf(t, madeUp)
	content := "clean\nzorvex quimby\nclean\r\nZorvex_Quimby\nzorvex\nquimby\n"
	got := l.MatchLines([]byte(content))
	assert.Equal(t, []int{2, 4}, got)
}

func TestNewEntryRefuses(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"spaces only", "   "},
		{"a hyphen", "zorvex-quimby"},
		{"a slash", "zorvex/quimby"},
		{"a dot", "zorvex.quimby"},
		{"a non-ASCII letter", "zorv\u00e9x"},
		{"a tab", "zorvex\tquimby"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewEntry(tt.input)
			require.Error(t, err)
			if name := strings.TrimSpace(tt.input); name != "" {
				assert.NotContains(t, err.Error(), name, "the error repeats the name")
			}
		})
	}
}

func TestNewEntryLowercasesAndTrims(t *testing.T) {
	a, err := NewEntry("  ZORVEX   Quimby ")
	require.NoError(t, err)
	b, err := NewEntry(madeUp)
	require.NoError(t, err)
	l := &List{}
	require.True(t, l.Add(a), "first Add")
	assert.False(t, l.Add(b), "Add of the same entry twice")
	assert.Equal(t, 1, l.Len())
}

// TestFileHoldsNoName checks what the hash is for: the plaintext is not
// in the file.
func TestFileHoldsNoName(t *testing.T) {
	data := listOf(t, madeUp, "plimsor").Marshal()
	for _, piece := range []string{"zorvex", "quimby", "plimsor"} {
		assert.NotContains(t, strings.ToLower(string(data)), piece, "the marshalled denylist")
	}
	assert.True(t, strings.HasPrefix(string(data), "# SPDX-FileCopyrightText:"),
		"the marshalled denylist has no SPDX header:\n%s", data)
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "denylist.yaml")
	require.NoError(t, os.WriteFile(path, listOf(t, madeUp, "plimsor").Marshal(), 0o644))
	l, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, 2, l.Len())
	assert.True(t, l.Match([]byte("Zorvex-Quimby")), "a loaded list matches its own names")
	assert.True(t, l.Match([]byte("plimsor")), "a loaded list matches its own names")
}

// TestEntrySegments covers the count "hygiene add" reports: one per
// piece of the typed name, however many spaces stand between two.
func TestEntrySegments(t *testing.T) {
	for _, tt := range []struct {
		name string
		want int
	}{{"zorvex", 1}, {"zorvex quimby", 2}, {" zorvex   quimby  blorp ", 3}} {
		e, err := NewEntry(tt.name)
		require.NoError(t, err, "NewEntry(%q)", tt.name)
		assert.Equal(t, tt.want, e.Segments(), "NewEntry(%q).Segments()", tt.name)
	}
}

// TestMerge covers the list a check builds from two files: every entry
// of both, each once.
func TestMerge(t *testing.T) {
	l := listOf(t, "zorvex quimby")
	l.Merge(listOf(t, "zorvex quimby", "blorptang"))
	l.Merge(&List{})
	assert.Equal(t, 2, l.Len())
	for _, line := range []string{"a zorvex-quimby b", "a blorptang b"} {
		assert.True(t, l.Match([]byte(line)), "the merged list matches %q", line)
	}
	empty := &List{}
	empty.Merge(l)
	assert.Equal(t, 2, empty.Len(), "Len of a merge into the zero value")
}

func TestLoadMissing(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "denylist.yaml"))
	require.ErrorIs(t, err, fs.ErrNotExist, "Load of a missing file")
}

func TestParse(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	tests := []struct {
		name    string
		yaml    string
		entries int
		wantErr bool
	}{
		{"empty file", "", 0, false},
		{"no entries", "entries: []\n", 0, false},
		{"one entry", "entries:\n  - segments:\n      - length: 3\n        sha256: " + sum + "\n", 1, false},
		{"unknown key", "entries: []\nextra: 1\n", 0, true},
		{"entry without segments", "entries:\n  - segments: []\n", 0, true},
		{"zero length", "entries:\n  - segments:\n      - length: 0\n        sha256: " + sum + "\n", 0, true},
		{"short hash", "entries:\n  - segments:\n      - length: 3\n        sha256: abcd\n", 0, true},
		{"hash that is not hex", "entries:\n  - segments:\n      - length: 3\n        sha256: " + strings.Repeat("zz", 32) + "\n", 0, true},
		{"not YAML", "entries: [\n", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := Parse([]byte(tt.yaml))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.entries, l.Len())
		})
	}
}
