// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package all

import "github.com/brunovenceslau/romeu-e-julieta/internal/state"

// The formats of internal/state, the three host state records (03 3.8),
// and its two transition tables. They are here so that tools/ci, which
// reaches internal/ through this package (REQ-0971), registers the
// schemas and writes the diagrams.
type (
	// StateToolchain is the type of the format state-toolchain.v1.
	StateToolchain = state.Toolchain
	// StateProject is the type of the format state-project.v1.
	StateProject = state.Project
	// StateDescriptor is the type of the format state-descriptor.v1.
	StateDescriptor = state.Descriptor
	// StateMachine is a transition table of 01 1.6.
	StateMachine = state.Machine
)

// StateMachines are the two machines of 01 1.6, the candidate and the
// generation, whose diagrams are generated from their tables.
var StateMachines = []StateMachine{state.CandidateMachine, state.GenerationMachine}
