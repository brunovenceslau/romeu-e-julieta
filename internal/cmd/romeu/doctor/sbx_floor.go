// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"
	"errors"
	"os"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
)

// The check sbx-floor (04 4.2, REQ-0366): sbx is not at the absolute
// path of the settings, or its version is below the floor of 12 12.1
// (sbx-floor, RJ-343, the error every command that starts sbx raises,
// REQ-0441).
var _ = register(row{slug: "sbx-floor", run: checkSbxFloor})

func checkSbxFloor(ctx context.Context, h *host) Check {
	const slug = "sbx-floor"
	if h.baseErr != nil {
		return errorOf(slug, h.baseErr)
	}
	if info, err := os.Stat(h.settings.Tools.Sbx); err != nil || !info.Mode().IsRegular() {
		return fail(slug, "sbx is not at the path the settings give: "+h.settings.Tools.Sbx)
	}
	v, err := h.requireFloor(ctx)
	if e, ok := errors.AsType[*cli.Error](err); ok && e.ID == sbxdrv.SbxFloor.ID {
		return failOf(slug, err)
	}
	if err != nil {
		return errorOf(slug, err)
	}
	return pass(slug, "sbx "+v.String()+", floor "+sbxdrv.Floor)
}
