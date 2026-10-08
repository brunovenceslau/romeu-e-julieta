// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package slug

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSlug pins the rule of ADR 0001, rule 6, on its edges: case, runs of
// separators, trimming, digits, and letters outside ASCII.
func TestSlug(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{"plain words", "Adopt a documentation standard", "adopt-a-documentation-standard"},
		{"a run of separators is one hyphen", "Keep  the -- floor,  ask first", "keep-the-floor-ask-first"},
		{"trims both ends", "  --Decide now!?  ", "decide-now"},
		{"digits stay", "Adopt six XP practices in 2026", "adopt-six-xp-practices-in-2026"},
		{"punctuation inside a word separates", "Let the sandbox act as the maintainer on GitHub's side", "let-the-sandbox-act-as-the-maintainer-on-github-s-side"},
		{"a letter outside ASCII is a separator", "Café e KK", "cafe-e-k"},
		{"a multi-byte letter between words", "aéb", "a-b"},
		{"nothing to keep", "!?- é", ""},
		{"empty", "", ""},
		{"already a slug", "keep-it", "keep-it"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Slug(tt.title))
		})
	}
}

// TestSlugIsIdempotent checks that a slug is its own slug, so a filename
// built from one and read back does not move.
func TestSlugIsIdempotent(t *testing.T) {
	for _, title := range []string{"Adopt X, Y and Z", "9 lives", "a  b"} {
		once := Slug(title)
		assert.Equal(t, once, Slug(once), title)
	}
}
