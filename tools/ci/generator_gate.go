// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"os"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// The generator of the consumers of the gate and apply tags that live in
// internal/gate (12 12.3, REQ-1106): the field lists of the widening set
// and of the recreate digest, and the I7 flip table. It fails on a field
// that is on neither the tags nor the committed list of ungated fields.
func init() {
	check.RegisterGenerator(check.Generator{Name: "gate field lists", Outputs: gateOutputs})
}

// gateOutputs returns the generated files of internal/gate.
func gateOutputs(*os.Root) ([]check.Output, error) {
	files, err := all.GateGenerated()
	if err != nil {
		return nil, err
	}
	outs := make([]check.Output, 0, len(files))
	for _, f := range files {
		outs = append(outs, check.Output{Path: f.Path, Content: f.Content})
	}
	return outs, nil
}
