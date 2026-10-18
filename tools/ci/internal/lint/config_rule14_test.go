// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lint

// rule14Fixtures holds one violation of each checker of ADR 0001, rule
// 14 (revive's exported and error-strings, gocritic's commentedOutCode),
// and one of the default linters that the repository relies on in
// generated files too (ADR 0016: unused, staticcheck).
func rule14Fixtures() []lintFixture {
	return []lintFixture{
		{
			file:    "doccomment.go",
			source:  "package fixture\n\nfunc Undocumented() {}\n",
			linter:  "revive",
			message: "exported function Undocumented should have comment",
		},
		{
			// Several words that end with a full stop: revive's
			// error-strings and staticcheck's ST1005 both report it.
			// revive alone skips a one-word capitalized string.
			file: "errorstring.go",
			source: `package fixture

import "errors"

func errorString() error {
	return errors.New("Capitalized and ends with a full stop.")
}

var _ = errorString
`,
			linter:  "revive",
			message: "error strings should not be capitalized",
		},
		{
			file: "commentedcode.go",
			source: `package fixture

func commentedCode(n int) int {
	// total := n * 2
	// if total > 10 { return total }
	return n
}

var _ = commentedCode
`,
			linter:  "gocritic",
			message: "may want to remove commented-out code",
		},
		{
			// unused counts every object of a generated file as used
			// unless generated-is-used is false (ADR 0016).
			file:    "deadcode.go",
			source:  "package fixture\n\nfunc deadCode() {}\n",
			linter:  "unused",
			message: "func deadCode is unused",
		},
		{
			// A staticcheck check that, unlike the ones it filters in
			// generated files (ADR 0016), reads a file with the
			// generated header.
			file: "nilmap.go",
			source: `package fixture

func nilMap() {
	var m map[string]int
	m["a"] = 1
}

var _ = nilMap
`,
			linter:  "staticcheck",
			message: "SA5000",
		},
	}
}
