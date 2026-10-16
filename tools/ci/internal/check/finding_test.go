// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// TestFindingLines checks how findings print: one per line, the path
// quoted when it does not print, and the summary line.
func TestFindingLines(t *testing.T) {
	findings := []check.Finding{
		{Where: "a.md:1", Rule: "em-dash", Msg: "a dash"},
		{Where: "b\x1b.md", Rule: "name", Msg: "a name"},
	}
	assert.Equal(t, "a.md:1: em-dash: a dash\n\"b\\x1b.md\": name: a name\n", check.Lines(findings))
	assert.Equal(t, "hygiene: ok", check.Summary("hygiene", 0))
	assert.Equal(t, "hygiene: 2 finding(s)", check.Summary("hygiene", len(findings)))
}
