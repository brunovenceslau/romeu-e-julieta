// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lint

// errorlintFixtures holds the violation of errorlint: a comparison with
// == that fails on a wrapped error.
func errorlintFixtures() []lintFixture {
	return []lintFixture{{
		file: "errorlint.go",
		source: `package fixture

import "io"

func atEOF(err error) bool {
	return err == io.EOF
}

var _ = atEOF
`,
		linter:  "errorlint",
		message: "comparing with == will fail on wrapped errors",
	}}
}
