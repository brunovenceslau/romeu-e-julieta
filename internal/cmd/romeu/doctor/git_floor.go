// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"
	"errors"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
)

// The check git-floor (04 4.2, REQ-0368): the host git is below the
// floor of 05 5.1, or its version cannot be read (git-floor, RJ-344,
// gitsafe's check, which every command that starts git runs,
// REQ-0442).
var _ = register(row{slug: "git-floor", run: checkGitFloor})

func checkGitFloor(ctx context.Context, h *host) Check {
	const slug = "git-floor"
	if h.baseErr != nil {
		return errorOf(slug, h.baseErr)
	}
	err := h.gitFloor(&h.settings, h.home)(ctx)
	if _, ok := errors.AsType[*gitsafe.FloorError](err); ok {
		return failOf(slug, err)
	}
	if err != nil {
		return errorOf(slug, err)
	}
	return pass(slug, "git at or above the floor "+gitsafe.Floor())
}
