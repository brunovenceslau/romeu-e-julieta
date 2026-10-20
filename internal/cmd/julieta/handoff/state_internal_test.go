// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"

	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks"
)

// TestFailureLineEscapesTheHook: a hook name a store file carries cannot
// put a control sequence or a line feed in the print.
func TestFailureLineEscapesTheHook(t *testing.T) {
	got := failureLine(hooks.Failure{At: time.Unix(0, 0), Hook: "pre\x1b[2J\nforged", Repo: "r", Message: "m"})
	assert.NotContains(t, got, "\x1b")
	assert.NotContains(t, got, "\n")
}

// TestCapMessage cuts a long message at a character, with a mark.
func TestCapMessage(t *testing.T) {
	assert.Equal(t, "short", capMessage("short"))
	long := strings.Repeat("é", 400)
	got := capMessage(long)
	assert.True(t, utf8.ValidString(got))
	assert.LessOrEqual(t, len(got), maxMessage+3)
	assert.True(t, strings.HasSuffix(got, "..."))
}
