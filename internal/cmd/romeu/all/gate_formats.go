// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package all

import "github.com/brunovenceslau/romeu-e-julieta/internal/gate"

// GateGeneratedFile is one generated file of internal/gate: the field
// lists of the widening set and of the recreate digest, and the I7 flip
// table, which the gate and apply tags of internal/spec define (12
// 12.3, REQ-1106). They are here so that tools/ci, which reaches
// internal/ through this package (REQ-0971), writes them.
type GateGeneratedFile = gate.GeneratedFile

// GateGenerated returns the generated files of internal/gate; it fails
// on a field of the project spec that is on neither the gate tags nor
// the committed list of ungated fields (I7).
var GateGenerated = gate.Generated
