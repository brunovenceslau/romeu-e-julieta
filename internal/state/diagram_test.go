// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMachine_Diagram(t *testing.T) {
	for _, mc := range machines {
		t.Run(mc.m.Name(), func(t *testing.T) {
			d := mc.m.Diagram()
			assert.Equal(t, "docs/diagrams/"+mc.m.Name()+".md", mc.m.DiagramPath())
			assert.Contains(t, d, "```mermaid\nstateDiagram-v2\n")
			for _, r := range mc.m.Rows() {
				from, to := "[*]", "[*]"
				if r.From != None {
					from = strings.ReplaceAll(string(r.From), "-", "_")
				}
				if r.To != None {
					to = strings.ReplaceAll(string(r.To), "-", "_")
				}
				assert.Contains(t, d, "    "+from+" --> "+to+": "+string(r.Event)+" ("+r.Req+")\n", "an arrow for %s", r.Req)
				assert.Contains(t, d, "| `"+string(r.Event)+"` |", "a table row for %s", r.Req)
				assert.Contains(t, d, string(r.Effect), "the effect of %s", r.Req)
			}
			for _, s := range mc.m.States() {
				if s != None {
					assert.Contains(t, d, `state "`+string(s)+`" as `, "the state %s is declared", s)
				}
			}
		})
	}
}
