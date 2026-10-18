// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lint

// usetestingFixtures holds the violation of usetesting: a test that
// makes its own temporary directory.
func usetestingFixtures() []lintFixture {
	return []lintFixture{{
		file: "usetesting_test.go",
		source: `package fixture

import (
	"os"
	"testing"
)

func TestTempDir(t *testing.T) {
	_, _ = os.MkdirTemp("", "fixture")
}
`,
		linter:  "usetesting",
		message: "os.MkdirTemp() could be replaced by t.TempDir()",
	}}
}
