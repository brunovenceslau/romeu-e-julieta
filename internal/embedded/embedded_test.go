// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package embedded

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSourceBuildEmbedsNoJulieta: a build without the overlay of
// tools/release build embeds the empty placeholder (REQ-0130), and the
// caller gets a copy, so a change to it leaves the embedded bytes alone.
func TestSourceBuildEmbedsNoJulieta(t *testing.T) {
	assert.Empty(t, Julieta())

	old := julieta
	julieta = []byte("a julieta")
	t.Cleanup(func() { julieta = old })
	got := Julieta()
	got[0] = 'A'
	assert.Equal(t, []byte("a julieta"), Julieta())
}
