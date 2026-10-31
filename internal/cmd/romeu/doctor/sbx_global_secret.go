// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import "context"

// The check sbx-global-secret, marked pre in 04 4.2: the global sbx
// secrets for a service a project uses (I22, REQ-0371). It is the check
// of step 5 of the preflight (internal/preflight), called from here.
var _ = register(row{slug: slugGlobalSecret, run: checkSbxGlobalSecret})

func checkSbxGlobalSecret(ctx context.Context, h *host) Check {
	return h.preCheck(ctx, slugGlobalSecret)
}
