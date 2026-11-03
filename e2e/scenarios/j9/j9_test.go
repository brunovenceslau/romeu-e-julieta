// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package j9_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios"
	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios/j9"
)

// TestJ9ReadsEveryLine is the completeness test of ADR 0001 rule 13
// for this journey: the function reads every line of commands.yaml, its
// steps are whole, and it runs nothing.
func TestJ9ReadsEveryLine(t *testing.T) {
	tbl, err := scenarios.Load("j9")
	require.NoError(t, err)

	s := j9.J9(tbl, j9.Vars{NewSHA256: "0"})

	assert.Empty(t, tbl.Unread(), "a commands.yaml line no function of J9 reads")
	require.NoError(t, s.Check())
	assert.Equal(t, []string{"romeu upgrade"}, s.Lines())
}
