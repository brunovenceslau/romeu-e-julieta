// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package prose

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixture is a made-up list: the tests of the matcher do not depend on
// the words the repository really bans.
const fixture = `
rules:
  - id: shiny
    phrases:
      - glimmerous
      - truly zappy
    fails:
      - a glimmerous tool
      - it is truly zappy
    passes:
      - a plain tool
  - id: pair
    pairs:
      - [either way, or else]
    fails:
      - either way we go, or else we stay
    passes:
      - or else we stay, either way
`

func mustParse(t *testing.T, data string) *Rules {
	t.Helper()
	r, err := Parse([]byte(data))
	require.NoError(t, err, "Parse")
	return r
}

func TestFind(t *testing.T) {
	r := mustParse(t, fixture)
	fence := "```"
	tests := []struct {
		name string
		text string
		want []Hit
	}{
		{"clean", "a plain tool\n", nil},
		{"a word", "a glimmerous tool\n", []Hit{{1, "shiny"}}},
		{"ASCII case ignored", "A GLIMMEROUS tool\n", []Hit{{1, "shiny"}}},
		{"whole words only", "a glimmerously made tool, unglimmerous\n", nil},
		{"next to punctuation", "is it (glimmerous)?\n", []Hit{{1, "shiny"}}},
		{"a phrase", "it is truly zappy\n", []Hit{{1, "shiny"}}},
		{"a phrase across a line end", "it is truly\nzappy today\n", []Hit{{1, "shiny"}}},
		{"a phrase across a blank line", "it is truly\n\nzappy today\n", nil},
		{"a phrase with a word between", "truly very zappy\n", nil},
		{"the line of the hit", "one\ntwo\n\nthree glimmerous\n", []Hit{{4, "shiny"}}},
		{"two hits", "glimmerous\n\nglimmerous\n", []Hit{{1, "shiny"}, {3, "shiny"}}},
		{"inside a code span", "the word `glimmerous` is listed\n", nil},
		{"inside a double code span", "the word ``a ` glimmerous`` is listed\n", nil},
		{"a code span across a line end", "the words `truly\nglimmerous` are listed\n", nil},
		{"after a code span", "`x` is glimmerous\n", []Hit{{1, "shiny"}}},
		{"an unmatched backtick", "a ` glimmerous tool\n", []Hit{{1, "shiny"}}},
		{"inside a fenced block", fence + "\nglimmerous\n" + fence + "\n", nil},
		{"inside a tilde block", "~~~text\nglimmerous\n~~~\n", nil},
		{"after a fenced block", fence + "text\nx\n" + fence + "\nglimmerous\n", []Hit{{4, "shiny"}}},
		{"a longer closing fence", fence + "\nglimmerous\n" + fence + "`\nglimmerous\n", []Hit{{4, "shiny"}}},
		{"a shorter fence does not close", fence + "`\n" + fence + "\nglimmerous\n", nil},
		{"an indented fence", "   " + fence + "\nglimmerous\n   " + fence + "\n", nil},
		{"a pair", "either way we go, or else we stay\n", []Hit{{1, "pair"}}},
		{"a pair across lines", "x\neither way we go,\nor else we stay\n", []Hit{{2, "pair"}}},
		{"a pair in the wrong order", "or else we stay, either way\n", nil},
		{"a pair across paragraphs", "either way\n\nor else\n", nil},
		{"half a pair", "either way we go\n", nil},
		{"no final newline", "glimmerous", []Hit{{1, "shiny"}}},
		{"CR LF line ends", "a\r\nglimmerous\r\n", []Hit{{2, "shiny"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, r.Find([]byte(tt.text)), "Find(%q)", tt.text)
		})
	}
}

// TestSelfTest shows that a list whose own samples disagree with it
// does not load.
func TestSelfTest(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"a fails sample that passes", "rules:\n  - id: a\n    phrases: [zappy]\n    fails: [plain]\n    passes: [plain]\n", "fails sample"},
		{"a passes sample that fails", "rules:\n  - id: a\n    phrases: [zappy]\n    fails: [zappy]\n    passes: [so zappy]\n", "passes sample"},
		{"a passes sample another rule fails", "rules:\n  - id: a\n    phrases: [zappy]\n    fails: [zappy]\n    passes: [plain]\n  - id: b\n    phrases: [plain]\n    fails: [plain]\n    passes: [x]\n", "passes sample"},
		{"no fails sample", "rules:\n  - id: a\n    phrases: [zappy]\n    passes: [plain]\n", "fails sample"},
		{"no passes sample", "rules:\n  - id: a\n    phrases: [zappy]\n    fails: [zappy]\n", "passes sample"},
		{"a phrase no sample reaches", "rules:\n  - id: a\n    phrases: [zappy, zippy]\n    fails: [zappy]\n    passes: [plain]\n", "no fails sample"},
		{"a phrase without a word", "rules:\n  - id: a\n    phrases: ['...']\n    fails: [zappy]\n    passes: [plain]\n", "no word"},
		{"a pair with one part", "rules:\n  - id: a\n    pairs: [[zappy]]\n    fails: [zappy]\n    passes: [plain]\n", "two parts"},
		{"a rule without an id", "rules:\n  - phrases: [zappy]\n    fails: [zappy]\n    passes: [plain]\n", "id"},
		{"a repeated id", "rules:\n  - id: a\n    phrases: [zappy]\n    fails: [zappy]\n    passes: [plain]\n  - id: a\n    phrases: [zippy]\n    fails: [zippy]\n    passes: [plain]\n", "twice"},
		{"a rule that lists nothing", "rules:\n  - id: a\n    fails: [zappy]\n    passes: [plain]\n", "lists nothing"},
		{"no rule", "rules: []\n", "no rule"},
		{"unknown key", "rules: []\nextra: 1\n", "extra"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			require.ErrorContains(t, err, tt.want)
		})
	}
}

// TestCommittedList loads the list the repository really uses, which
// runs its self-test, and checks that it covers the four word rules of
// ADR 0001 rule 4.
func TestCommittedList(t *testing.T) {
	data, err := os.ReadFile("../prose.yaml")
	require.NoError(t, err)
	r := mustParse(t, string(data))
	want := []string{"hype", "ai-meta-commentary", "attribution-footer", "paired-construction"}
	assert.Equal(t, want, r.IDs(), "rule ids")
}
