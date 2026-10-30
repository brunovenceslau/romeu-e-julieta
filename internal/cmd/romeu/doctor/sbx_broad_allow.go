// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import "context"

// The check sbx-broad-allow, marked pre in 04 4.2: a global allow rule
// of ** or another broad wildcard, or a permissive default network
// policy (I22, REQ-0372). It is the check of step 5 of the preflight
// (internal/preflight), called from here.
var _ = register(row{slug: slugBroadAllow, run: checkSbxBroadAllow})

func checkSbxBroadAllow(ctx context.Context, h *host) Check { return h.preCheck(ctx, slugBroadAllow) }
