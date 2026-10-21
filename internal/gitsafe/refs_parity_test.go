// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

// TestCheckRefParity: romeu's reader of snapshot/heads.json
// (salvage.CheckRef) accepts a ref name below the namespaces of a
// snapshot exactly when gitsafe, which read it, does, so the writer
// never writes a name the reader refuses and the reader admits nothing
// gitsafe would not.
func TestCheckRefParity(t *testing.T) {
	for _, name := range []string{
		"refs/heads/main", "refs/heads/feature/x-1", "refs/tags/v1.0", "refs/salvage-reflog/main/3",
		"refs/heads/café", "refs/heads/a..b", "refs/heads/a@{1}", "refs/heads/a.", "refs/heads/a.lock",
		"refs/heads/.a", "refs/heads/a//b", "refs/heads/a b", "refs/heads/a~1", "refs/heads/a^", "refs/heads/a:b",
		"refs/heads/a?", "refs/heads/a[b", "refs/heads/a\\b", "refs/heads/a*", "refs/heads/a\x1bb", "refs/heads/a\x7fb",
		"refs/heads/a\u202eb", "refs/heads/a\u200db", "refs/heads/a\ufeffb", "refs/heads/a\u2028b", "refs/heads/a\u00adb",
		"refs/heads/a/", "refs/heads/@", "refs/heads/a.lock/b",
	} {
		g := gitsafe.CheckDisplayRef(name) == nil
		s := salvage.CheckRef(name) == nil
		assert.Equal(t, g, s, "%q: gitsafe accepts %v, salvage %v", name, g, s)
	}
}
