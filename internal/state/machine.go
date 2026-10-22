// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"fmt"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// State is a state of a machine of 01 1.6. None is the state of what
// has no record: the "(none)" of the tables.
type State string

// Event is what a command reports to a machine, such as "run-create".
// The events are the cells of the Event column of the tables in 01 1.6,
// each given a name.
type Event string

// Effect names what a row's transition does, the Effect column of the
// tables in 01 1.6. The effects at the level of the record are applied
// by Project.Apply and Candidate; the others are the command's, and the
// table names them so that the diagram and the effect case of each row
// stay with the row (REQ-0080).
type Effect string

// None is the state of a candidate or a generation that does not exist
// (01 1.6, the "(none)" cells).
const None State = "none"

// Row is one row of a transition table: from a state, on an event, to a
// state, with an effect (REQ-0079). Req is the requirement id of the
// row in the specification, which the diagram and the tests name.
type Row struct {
	From   State
	Event  Event
	To     State
	Effect Effect
	Req    string
}

// Machine is a transition table (REQ-0961). The two machines are the
// package-level CandidateMachine and GenerationMachine.
type Machine struct {
	name string
	// about is the sentence the generated diagram page opens with.
	about string
	rows  []Row
}

// Name returns the machine's name, "candidate" or "generation".
func (m Machine) Name() string { return m.name }

// Rows returns the rows of the table in the order of 01 1.6.
func (m Machine) Rows() []Row { return slices.Clone(m.rows) }

// States returns the states the table names, in the order of their first
// appearance, None first when the table has it.
func (m Machine) States() []State {
	var out []State
	for _, r := range m.rows {
		for _, s := range []State{r.From, r.To} {
			if !slices.Contains(out, s) {
				out = append(out, s)
			}
		}
	}
	return out
}

// Events returns the events the table names, in the order of their
// first appearance.
func (m Machine) Events() []Event {
	var out []Event
	for _, r := range m.rows {
		if !slices.Contains(out, r.Event) {
			out = append(out, r.Event)
		}
	}
	return out
}

// Transition returns the row of the pair (from, event). A pair with no
// row is an illegal transition: the error illegal-transition (RJ-350),
// exit 2, and nothing changes (REQ-0079).
func (m Machine) Transition(from State, event Event) (Row, error) {
	for _, r := range m.rows {
		if r.From == from && r.Event == event {
			return r, nil
		}
	}
	return Row{}, IllegalTransition.New(
		fmt.Sprintf("the %s has no transition from %q on %q", m.name, from, event),
		cli.Detail{Key: "machine", Value: m.name},
		cli.Detail{Key: "from", Value: string(from)},
		cli.Detail{Key: "event", Value: string(event)})
}
