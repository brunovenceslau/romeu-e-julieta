// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import "context"

// The check auto-trust, marked pre in 04 4.2: a mise
// trusted_config_paths entry or a direnv allow entry that covers
// $ROMEU_ROOT (I23, REQ-0379). It is the check of step 5 of the
// preflight (internal/preflight), called from here.
var _ = register(row{slug: slugAutoTrust, run: checkAutoTrust})

func checkAutoTrust(ctx context.Context, h *host) Check { return h.preCheck(ctx, slugAutoTrust) }
