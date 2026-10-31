// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"
	"errors"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/approve"
)

// The check toolchain-approval-required (04 4.2, REQ-0369): the
// toolchain acknowledgement of gate 1 is missing or stale. It is gate
// 1's own check, approve.Check of T042, over the live toolchain
// approve.Measure reads from the sbx version and this binary; a romeu
// built without julieta measures the placeholder it embeds, as approve
// records it. With no state directory no toolchain was acknowledged.
var _ = register(row{slug: "toolchain-approval-required", run: checkToolchain})

func checkToolchain(ctx context.Context, h *host) Check {
	const slug = "toolchain-approval-required"
	release, err := h.requireFloor(ctx)
	if err != nil {
		return errorOf(slug, err)
	}
	if h.stateErr != nil {
		return errorOf(slug, h.stateErr)
	}
	live, err := approve.Measure(release, h.env.Version)
	if err != nil {
		return errorOf(slug, err)
	}
	if h.store == nil {
		return fail(slug, "no toolchain is acknowledged on this machine (gate 1): there is no state directory yet")
	}
	err = approve.Check(h.store, live)
	if e, ok := errors.AsType[*cli.Error](err); ok && e.ID == approve.ToolchainApprovalRequired.ID {
		return failOf(slug, err)
	}
	if err != nil {
		return errorOf(slug, err)
	}
	return pass(slug, "the toolchain of this machine is acknowledged")
}
