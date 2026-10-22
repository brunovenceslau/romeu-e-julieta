// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWarningCommandEscapes holds the escaping rule of a workflow
// command: the message escapes %, CR and LF, and a property escapes ":"
// and "," as well (actions/toolkit, command.ts: escapeData,
// escapeProperty). A CR in a message is written "\r" by git.SafeLines
// before it gets there.
func TestWarningCommandEscapes(t *testing.T) {
	assert.Equal(t, "::warning title=e2e::plain: text, kept", warningCommand("e2e", "plain: text, kept"))
	assert.Equal(t, "::warning title=a%3Ab%2Cc%25d%0De%0Af::50%25 a\\r%0Ab", warningCommand("a:b,c%d\re\nf", "50% a\r\nb"))
	assert.Equal(t, "::warning title=t::%0A./::error::forged", warningCommand("t", "\n::error::forged"), "a line break in the message cannot start a command of its own")
	assert.NotContains(t, warningCommand("t", "x ##[error]y"), "##[", "the legacy marker is broken")
}
