// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package j11_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios"
	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios/j11"
)

// TestJ11ReadsEveryLine is the completeness test of ADR 0001 rule 13
// for this journey: the function reads every line of commands.yaml, its
// steps are whole, and it runs nothing.
func TestJ11ReadsEveryLine(t *testing.T) {
	tbl, err := scenarios.Load("j11")
	require.NoError(t, err)

	s := j11.J11(tbl, j11.Vars{Name: "foo"})

	assert.Empty(t, tbl.Unread(), "a commands.yaml line no function of J11 reads")
	require.NoError(t, s.Check())
	assert.Equal(t, []string{"romeu sync foo"}, s.Lines())
}
