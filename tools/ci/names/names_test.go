// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-or-later

package names

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// madeUp is the name every test here uses. It is an invented word pair
// and names nothing.
const madeUp = "zorvex quimby"

func listOf(t *testing.T, madeUpNames ...string) *List {
	t.Helper()
	l := &List{}
	for _, n := range madeUpNames {
		e, err := NewEntry(n)
		if err != nil {
			t.Fatalf("NewEntry: %v", err)
		}
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
			if got := l.Match([]byte(tt.line)); got != tt.want {
				t.Errorf("Match(%q) = %v, want %v", tt.line, got, tt.want)
			}
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
			if got := l.Match([]byte(tt.line)); got != tt.want {
				t.Errorf("Match(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestMatchLines(t *testing.T) {
	l := listOf(t, madeUp)
	content := "clean\nzorvex quimby\nclean\r\nZorvex_Quimby\nzorvex\nquimby\n"
	got := l.MatchLines([]byte(content))
	want := []int{2, 4}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("MatchLines = %v, want %v", got, want)
	}
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
			if err == nil {
				t.Fatalf("NewEntry(%q) succeeded, want an error", tt.input)
			}
			if tt.input != "" && strings.TrimSpace(tt.input) != "" &&
				strings.Contains(err.Error(), strings.TrimSpace(tt.input)) {
				t.Errorf("the error repeats the name: %v", err)
			}
		})
	}
}

func TestNewEntryLowercasesAndTrims(t *testing.T) {
	a, err := NewEntry("  ZORVEX   Quimby ")
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewEntry(madeUp)
	if err != nil {
		t.Fatal(err)
	}
	l := &List{}
	if !l.Add(a) {
		t.Fatal("first Add returned false")
	}
	if l.Add(b) {
		t.Error("Add accepted the same entry twice")
	}
	if l.Len() != 1 {
		t.Errorf("Len = %d, want 1", l.Len())
	}
}

// TestFileHoldsNoName checks what the hash is for: the plaintext is not
// in the file.
func TestFileHoldsNoName(t *testing.T) {
	data := listOf(t, madeUp, "plimsor").Marshal()
	for _, piece := range []string{"zorvex", "quimby", "plimsor"} {
		if strings.Contains(strings.ToLower(string(data)), piece) {
			t.Errorf("the marshalled denylist contains %q", piece)
		}
	}
	if !strings.HasPrefix(string(data), "# SPDX-FileCopyrightText:") {
		t.Errorf("the marshalled denylist has no SPDX header:\n%s", data)
	}
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "denylist.yaml")
	if err := os.WriteFile(path, listOf(t, madeUp, "plimsor").Marshal(), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if l.Len() != 2 {
		t.Fatalf("Len = %d, want 2", l.Len())
	}
	if !l.Match([]byte("Zorvex-Quimby")) || !l.Match([]byte("plimsor")) {
		t.Error("a loaded list does not match its own names")
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "denylist.yaml"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Load of a missing file = %v, want fs.ErrNotExist", err)
	}
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
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && l.Len() != tt.entries {
				t.Errorf("Len = %d, want %d", l.Len(), tt.entries)
			}
		})
	}
}
