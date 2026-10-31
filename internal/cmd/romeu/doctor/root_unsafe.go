// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"

	"github.com/brunovenceslau/romeu-e-julieta/internal/hostpaths"
)

// The check root-unsafe (04 4.2, REQ-0375, REQ-1277; I20): the root of
// the settings breaks a rule of the root check, which init applies
// before it writes and which internal/hostpaths holds for both
// (hostpaths.RootProblem, T169); the message names the rule.
var _ = register(row{slug: "root-unsafe", run: checkRootUnsafe})

func checkRootUnsafe(_ context.Context, h *host) Check {
	const slug = "root-unsafe"
	if h.baseErr != nil {
		return errorOf(slug, h.baseErr)
	}
	if h.stateErr != nil {
		return errorOf(slug, h.stateErr)
	}
	paths, err := hostpaths.RealPaths(h.home, h.settingsPath, h.stateDir)
	if err != nil {
		return errorOf(slug, err)
	}
	if rule := hostpaths.RootProblem(h.settings.Root, h.settings.Config.Project+"-env", paths); rule != "" {
		return fail(slug, "the root "+h.settings.Root+": "+rule)
	}
	return pass(slug, h.settings.Root)
}
