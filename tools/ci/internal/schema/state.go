// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schema

import (
	"os"
	"reflect"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/schemagen"
)

// The formats of the host state records (03 3.8), whose Go types live in
// internal/state; tools/ci reaches them through internal/cmd/romeu/all
// (REQ-0971).
func init() {
	schemagen.Register(schemagen.Schema{Name: "state-toolchain.v1", Type: reflect.TypeFor[all.StateToolchain]()})
	schemagen.Register(schemagen.Schema{Name: "state-project.v1", Type: reflect.TypeFor[all.StateProject]()})
	schemagen.Register(schemagen.Schema{Name: "state-descriptor.v1", Type: reflect.TypeFor[all.StateDescriptor]()})
}

// DiagramOutputs returns the diagram page of each state machine, one
// file each under docs/diagrams/ (12 12.3, REQ-1110), which
// ARCHITECTURE.md links.
func DiagramOutputs(*os.Root) ([]check.Output, error) {
	var outs []check.Output
	for _, m := range all.StateMachines {
		page := "---\ntype: reference\nreader: a contributor or reviewer who reads how a state machine of romeu moves\n---\n\n" +
			check.HTMLComment(check.GeneratedHeader("internal/state (the transition table of the "+m.Name()+" machine)")) + "\n" +
			m.Diagram()
		outs = append(outs, check.Output{Path: m.DiagramPath(), Content: []byte(page)})
	}
	return outs, nil
}
