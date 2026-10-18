// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lint

// packageCommentsFixtures holds the violation of revive's
// package-comments rule: a package with no package comment. It is a
// package of its own, because the comment of one file serves the whole
// package, and clean.go carries the one of the package fixture.
func packageCommentsFixtures() []lintFixture {
	return []lintFixture{{
		file:    "nocomment/doc.go",
		source:  "package nocomment\n",
		linter:  "revive",
		message: "should have a package comment",
	}}
}
