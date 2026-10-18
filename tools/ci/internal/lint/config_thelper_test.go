// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lint

// thelperFixtures holds the violations of thelper: a test helper that
// does not start with t.Helper(), and one that names its testing.TB
// parameter t and not tb.
func thelperFixtures() []lintFixture {
	return []lintFixture{
		{
			file: "thelper_test.go",
			source: `package fixture

import "testing"

func requireFine(t *testing.T) {
	t.Log("fine")
}

func TestFine(t *testing.T) {
	requireFine(t)
}
`,
			linter:  "thelper",
			message: "test helper function should start from t.Helper()",
		},
		{
			file: "thelpertb_test.go",
			source: `package fixture

import "testing"

func requireTB(t testing.TB) {
	t.Helper()
}

func TestTB(t *testing.T) {
	requireTB(t)
}
`,
			linter:  "thelper",
			message: "parameter testing.TB should have name tb",
		},
	}
}
