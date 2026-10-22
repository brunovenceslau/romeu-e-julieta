// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/schema"
)

// The generator of the state machine diagrams (12 12.3, REQ-1110): one
// file for each machine of internal/state under docs/diagrams/, which
// ARCHITECTURE.md links.
func init() {
	check.RegisterGenerator(check.Generator{Name: "state diagrams", Outputs: schema.DiagramOutputs})
}
