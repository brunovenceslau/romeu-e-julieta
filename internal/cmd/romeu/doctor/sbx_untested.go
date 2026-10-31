// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
)

// The check sbx-untested (04 4.2, REQ-0367): the live sbx version is
// outside the tested window of 10 10.3, a warning that names the
// version and the recorded range. Until block A records sessions the
// window is the versions the synthetic sessions were written for,
// sbxdrv.RecordedVersions (REQ-0932). A version below the floor is
// still judged: it is read, only not driven.
var _ = register(row{slug: "sbx-untested", run: checkSbxUntested})

func checkSbxUntested(ctx context.Context, h *host) Check {
	const slug = "sbx-untested"
	v, err := h.requireFloor(ctx)
	if e, ok := errors.AsType[*cli.Error](err); err != nil && (!ok || e.ID != sbxdrv.SbxFloor.ID) {
		return errorOf(slug, err)
	}
	window := strings.Join(sbxdrv.RecordedVersions, ", ")
	if !slices.Contains(sbxdrv.RecordedVersions, v.String()) {
		return warn(slug, "sbx "+v.String()+" is outside the tested window, the recorded versions "+window)
	}
	return pass(slug, "sbx "+v.String()+" is a recorded version ("+window+")")
}
