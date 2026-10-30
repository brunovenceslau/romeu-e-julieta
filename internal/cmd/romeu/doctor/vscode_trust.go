// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/preflight"
)

// The check vscode-trust, marked pre in 04 4.2 (I14, I23, REQ-0380): VS
// Code workspace trust turned off, or $ROMEU_ROOT, a parent of it or a
// folder under it trusted. It is the check of step 5 of the preflight,
// called from here. The trusted folders live in VS Code's SQLite state
// database, which the preflight does not read (UncheckedVSCodeFolders),
// so a pass says they were not checked. Deferred decision, reopened at
// T049 by step5.go and kept: doctor passes no reader of the trusted
// folders, since reading SQLite is a dependency; the trigger is the
// first task that adds an SQLite reader, or T134.
var _ = register(row{slug: slugVSCodeTrust, run: checkVSCodeTrust})

func checkVSCodeTrust(ctx context.Context, h *host) Check {
	c := h.preCheck(ctx, slugVSCodeTrust)
	if c.Result != resultPass {
		return c
	}
	s, _ := h.findings.get(func() (stepFive, error) { return h.stepFive(ctx), nil })
	if slices.Contains(s.unchecked, preflight.UncheckedVSCodeFolders) {
		c.Message = "workspace trust is on; the trusted folders of VS Code are not checked, since they live in its state database"
	}
	return c
}
