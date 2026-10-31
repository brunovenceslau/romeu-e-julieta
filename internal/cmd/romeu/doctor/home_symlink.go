// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import "context"

// The check home-symlink, marked pre in 04 4.2: a symbolic link under
// $HOME (depth 1) or $HOME/.config (depth 2) that resolves into
// $ROMEU_ROOT (I23, REQ-0378). It is the check of step 5 of the
// preflight (internal/preflight), called from here.
var _ = register(row{slug: slugHomeSymlink, run: checkHomeSymlink})

func checkHomeSymlink(ctx context.Context, h *host) Check { return h.preCheck(ctx, slugHomeSymlink) }
