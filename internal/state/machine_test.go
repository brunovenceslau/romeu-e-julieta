// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// machines are the two machines of 01 1.6, with the number of rows each
// table holds in the specification.
var machines = []struct {
	m    Machine
	rows int
}{
	{CandidateMachine, 6},
	{GenerationMachine, 20},
}

// TestMachine_Transition is the generated test of REQ-0079 (it reads the
// tables, so it cannot lag a row): every row gives its To, and every
// (from, event) pair with no row is an illegal transition, exit 2 with
// its error id.
func TestMachine_Transition(t *testing.T) {
	for _, mc := range machines {
		t.Run(mc.m.Name()+" rows", func(t *testing.T) {
			require.Len(t, mc.m.Rows(), mc.rows, "the rows of the table in 01 1.6")
			for _, row := range mc.m.Rows() {
				t.Run(row.Req+" "+string(row.From)+" "+string(row.Event), func(t *testing.T) {
					got, err := mc.m.Transition(row.From, row.Event)
					require.NoError(t, err)
					assert.Equal(t, row, got)
					assert.NotEmpty(t, row.To)
					assert.NotEmpty(t, row.Effect)
				})
			}
		})
		t.Run(mc.m.Name()+" illegal pairs", func(t *testing.T) {
			legal := map[[2]string]bool{}
			for _, row := range mc.m.Rows() {
				legal[[2]string{string(row.From), string(row.Event)}] = true
			}
			n := 0
			// The pairs include an event of the machine from every
			// state, and a state and an event the machine does not have.
			from := append(mc.m.States(), "no-such-state")
			events := append(mc.m.Events(), "no-such-event")
			for _, f := range from {
				for _, e := range events {
					if legal[[2]string{string(f), string(e)}] {
						continue
					}
					n++
					t.Run(fmt.Sprintf("%s %s", f, e), func(t *testing.T) {
						_, err := mc.m.Transition(f, e)
						require.Error(t, err)
						assert.Equal(t, "RJ-350", cli.Of(err).ID)
						assert.Equal(t, 2, cli.ExitCode(err))
					})
				}
			}
			assert.Positive(t, n)
		})
	}
}

func TestMachine_tables(t *testing.T) {
	for _, mc := range machines {
		t.Run(mc.m.Name(), func(t *testing.T) {
			seen := map[[2]string]bool{}
			reqs := map[string]bool{}
			for _, row := range mc.m.Rows() {
				key := [2]string{string(row.From), string(row.Event)}
				assert.False(t, seen[key], "the pair %v has one row", key)
				seen[key] = true
				assert.False(t, reqs[row.Req], "%s names one row", row.Req)
				reqs[row.Req] = true
				assert.True(t, slices.Contains(mc.m.States(), row.From), "%s: from is a state", row.Req)
				assert.True(t, slices.Contains(mc.m.States(), row.To), "%s: to is a state", row.Req)
			}
		})
	}
}
