// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"fmt"
	"strings"
)

// DiagramPath returns the path, from the root of the repository, of the
// generated diagram of the machine: one file for each, under
// docs/diagrams/ (12 12.3, REQ-1110).
func (m Machine) DiagramPath() string { return "docs/diagrams/" + m.name + ".md" }

// Diagram returns the body of the machine's diagram page: a Mermaid
// state diagram of its table, then the table itself, since an effect is
// too long for an arrow. The page is generated from the table, so the
// diagram cannot disagree with the rows the tests exercise
// (REQ-0079, REQ-1110); tools/ci writes its header and front matter.
func (m Machine) Diagram() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# The %s state machine\n\n%s\n\n", m.name, m.about)
	b.WriteString("```mermaid\nstateDiagram-v2\n")
	for _, s := range m.States() {
		if s != None {
			fmt.Fprintf(&b, "    state %q as %s\n", s, nodeID(s))
		}
	}
	for _, r := range m.rows {
		fmt.Fprintf(&b, "    %s --> %s: %s (%s)\n", node(r.From), node(r.To), r.Event, r.Req)
	}
	b.WriteString("```\n\n")
	b.WriteString("An arrow reads `event (requirement)`. `[*]` is none: no record.\n\n")
	b.WriteString("| From | Event | To | Effect | Requirement |\n|---|---|---|---|---|\n")
	for _, r := range m.rows {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s | %s |\n", r.From, r.Event, r.To, "`"+string(r.Effect)+"`", r.Req)
	}
	return b.String()
}

// node returns the Mermaid name of a state: [*] for none.
func node(s State) string {
	if s == None {
		return "[*]"
	}
	return nodeID(s)
}

// nodeID returns an identifier for a state name that holds a dash.
func nodeID(s State) string { return strings.ReplaceAll(string(s), "-", "_") }
