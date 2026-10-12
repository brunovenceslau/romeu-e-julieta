// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const oneMutant = `mutants:
  - id: flip
    file: calc/calc.go
    old: "x < lo"
    new: "x > lo"
    package: ./calc
    run: ^TestClamp$
`

// TestParseMutantsReadsEveryField is the acceptance case of the format.
func TestParseMutantsReadsEveryField(t *testing.T) {
	got, err := parseMutants([]byte(oneMutant))
	require.NoError(t, err)
	assert.Equal(t, []mutant{{ID: "flip", File: "calc/calc.go", Old: "x < lo", New: "x > lo", Package: "./calc", Run: "^TestClamp$"}}, got)
}

// TestParseMutantsAllowsAnEmptyNew checks that a mutant may delete the
// snippet, and that a package pattern with ... is a target.
func TestParseMutantsAllowsAnEmptyNew(t *testing.T) {
	got, err := parseMutants([]byte("mutants:\n  - {id: drop, file: a.go, old: x, new: '', package: ./..., run: .}\n"))
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Empty(t, got[0].New)
}

// TestParseMutantsRefuses covers every refusal: each names what is
// wrong, so a typing error never loads as a mutant that tests nothing.
func TestParseMutantsRefuses(t *testing.T) {
	entry := func(fields string) string { return "mutants:\n  - {" + fields + "}\n" }
	const ok = "file: a.go, old: x, new: y, package: ./a, run: ."
	tests := []struct {
		name, src, want string
	}{
		{"an empty file", "", "the mutants file is empty"},
		{"no mutant", "mutants: []\n", "lists no mutant"},
		{"an unknown key", entry("id: a, " + ok + ", extra: 1"), "field extra not found"},
		{"an unknown top key", "mutant: []\n", "field mutant not found"},
		{"a second document", oneMutant + "---\nmutants: []\n", "more than one YAML document"},
		{"not YAML", "mutants: [\n", "decode the mutants file"},
		{"no id", entry(ok), `the id ""`},
		{"an id with a space", entry("id: a b, " + ok), `the id "a b"`},
		{"an id that starts with a hyphen", entry("id: -a, " + ok), `the id "-a"`},
		{"a twice listed id", "mutants:\n  - {id: a, " + ok + "}\n  - {id: a, " + ok + "}\n", `the id "a" is listed twice`},
		{"no file", entry("id: a, old: x, new: y, package: ./a, run: ."), "file is empty"},
		{"an absolute file", entry("id: a, file: /etc/passwd, old: x, new: y, package: ./a, run: ."), "not a clean path"},
		{"a file above the top", entry("id: a, file: ../a.go, old: x, new: y, package: ./a, run: ."), "not a clean path"},
		{"an unclean file", entry("id: a, file: a//b.go, old: x, new: y, package: ./a, run: ."), "not a clean path"},
		{"a file with a dot segment", entry("id: a, file: ./a.go, old: x, new: y, package: ./a, run: ."), "not a clean path"},
		{"a file with a backslash", entry(`id: a, file: 'a\b.go', old: x, new: y, package: ./a, run: .`), "holds a backslash"},
		{"a file with a newline", entry(`id: a, file: "a\nb.go", old: x, new: y, package: ./a, run: .`), "holds a backslash"},
		{"an empty old", entry("id: a, file: a.go, old: '', new: y, package: ./a, run: ."), "old is empty"},
		{"new the same as old", entry("id: a, file: a.go, old: x, new: x, package: ./a, run: ."), "new is the same as old"},
		{"a package without ./", entry("id: a, file: a.go, old: x, new: y, package: a, run: ."), "does not start with ./"},
		{"a package that is a flag", entry("id: a, file: a.go, old: x, new: y, package: -exec=x, run: ."), "does not start with ./"},
		{"a package above the top", entry("id: a, file: a.go, old: x, new: y, package: ./../a, run: ."), "not a clean path"},
		{"an empty package path", entry("id: a, file: a.go, old: x, new: y, package: ./, run: ."), "package is empty"},
		{"no run", entry("id: a, file: a.go, old: x, new: y, package: ./a"), "run is empty"},
		{"a run with a newline", entry(`id: a, file: a.go, old: x, new: y, package: ./a, run: "a\nb"`), "does not print"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseMutants([]byte(tt.src))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// TestTargetsAreDistinctInOrder checks that each target runs one
// baseline, in the order the mutants first name it.
func TestTargetsAreDistinctInOrder(t *testing.T) {
	ms := []mutant{
		{Package: "./b", Run: "x"},
		{Package: "./a", Run: "x"},
		{Package: "./b", Run: "x"},
		{Package: "./b", Run: "y"},
	}
	assert.Equal(t, []target{{"./b", "x"}, {"./a", "x"}, {"./b", "y"}}, targets(ms))
}

// TestOccurrences checks that overlapping matches count: "xx" is not
// unique in "xxx", where either of two places could be replaced.
func TestOccurrences(t *testing.T) {
	assert.Equal(t, 2, occurrences([]byte("xxx"), []byte("xx")))
	assert.Equal(t, 1, occurrences([]byte("xx"), []byte("xx")))
	assert.Equal(t, 3, occurrences([]byte("aaaa"), []byte("aa")))
	assert.Equal(t, 2, occurrences([]byte("ab ab"), []byte("ab")))
	assert.Equal(t, 0, occurrences([]byte("a"), []byte("ab")))
}
