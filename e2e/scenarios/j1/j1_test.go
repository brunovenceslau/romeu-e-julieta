// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package j1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios"
	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios/j1"
)

// TestJ1ReadsEveryLine is the completeness test of ADR 0001 rule 13
// for this journey: the function reads every line of commands.yaml, its
// steps are whole, and it runs nothing.
func TestJ1ReadsEveryLine(t *testing.T) {
	tbl, err := scenarios.Load("j1")
	require.NoError(t, err)

	s := j1.J1(tbl, j1.Vars{Owner: "owner", ConfigRepo: "cfg"})

	assert.Empty(t, tbl.Unread(), "a commands.yaml line no function of J1 reads")
	assert.NoError(t, s.Check())
	assert.Equal(t, []string{"romeu init --config-url https://github.com/owner/cfg"}, s.Lines())
}
