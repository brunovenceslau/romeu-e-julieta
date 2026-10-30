// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package status

import (
	"context"
	"fmt"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// The sandbox section: the sandbox's state by sbx ls --json, and
// removal-pending, a removing generation whose sandbox is still present
// (04 4.2, REQ-0390), which only rm finishes.
var _ = register(section{
	name:  "sandbox",
	order: 10,
	fill: func(_ context.Context, src *source, p *Project) {
		if !src.listed {
			p.Sandbox = SandboxUnknown
			return
		}
		sb, ok := src.sandbox()
		if !ok {
			p.Sandbox = SandboxAbsent
			return
		}
		p.Sandbox = sb.Status
		if src.record == nil || len(src.record.Generations) == 0 {
			return
		}
		if g := src.record.Generations[len(src.record.Generations)-1]; g.State == state.GenerationRemoving {
			p.Findings = append(p.Findings, Finding{
				Slug:    "removal-pending",
				Message: fmt.Sprintf("generation %s is removing and its sandbox is present; run romeu rm %s", g.ID, src.name),
			})
		}
	},
	text: func(out *cli.Writer, p *Project) error {
		_, err := fmt.Fprintf(out, "  sandbox: %s\n", line(p.Sandbox))
		return err
	},
})
