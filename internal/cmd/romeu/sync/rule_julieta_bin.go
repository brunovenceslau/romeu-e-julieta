// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"context"

	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// The rule julieta-bin-reserved: julietaBin is the kit arg romeu
// renders, the absolute path of the project's julieta (06 6.4,
// REQ-0596), so a spec that sets it, on any kit, is a sync error that
// names the key (usage, exit 2). Render refuses it too, but only on a
// kit whose descriptor declares the arg, and as an error of no row.
var _ = register(rule{name: "julieta-bin-reserved", check: checkJulietaBin})

// checkJulietaBin refuses the first kit of a target whose spec sets
// julietaBin.
func checkJulietaBin(_ context.Context, r *run) error {
	for _, p := range r.targets {
		for _, k := range p.Kits {
			if _, ok := k.Args[render.JulietaBinArg]; ok {
				return usage("julieta-bin-reserved", "the spec of "+termsafe.Escape(p.Name)+" sets the arg "+render.JulietaBinArg+
					" of the kit "+termsafe.Escape(k.Product+k.Personal)+"; romeu renders that arg, and a spec does not set it")
			}
		}
	}
	return nil
}
