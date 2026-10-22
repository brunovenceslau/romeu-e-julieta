// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// TestBuildersCarryTheirClass holds the rule of 04 4.1 and the
// acceptance of T029: every builder names a row of the timeout table
// and says whether its call is interactive or captured; the argv and
// the shape have the same length, and the shape holds a placeholder
// only where the argv holds a value.
func TestBuildersCarryTheirClass(t *testing.T) {
	require.NotEmpty(t, examples())
	for _, c := range examples() {
		t.Run(c.Name+" "+shapeKey(c.Shape), func(t *testing.T) {
			assert.NotEmpty(t, c.Name)
			assert.NotEmpty(t, c.Argv)
			assert.Contains(t, []Mode{Captured, Interactive}, c.Mode)
			if c.Class != procgroup.Unbounded {
				assert.NotZero(t, c.Class.Timeout(), "class %s", c.Class)
			}
			assert.NoError(t, Guard(c))
		})
	}
}

// TestInteractiveCalls lists the interactive calls, which are the ones
// sbx prompts in (REQ-0270): everything else is captured.
func TestInteractiveCalls(t *testing.T) {
	var interactive []string
	for _, c := range examples() {
		if c.Mode == Interactive {
			interactive = append(interactive, c.Name)
		}
	}
	assert.ElementsMatch(t, wantInteractive, interactive)
}

// wantInteractive are the builders whose calls are interactive (stdio
// inherited, raw): sbx env run -d [--clone], and the env exec -it of
// the run layout and the shell.
var wantInteractive = []string{}
