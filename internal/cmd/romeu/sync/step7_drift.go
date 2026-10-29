// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import "context"

// checkDrift is step 7 of 04 4.2: the drift check of the live files
// against the current render.json (REQ-0342). Lane D of T043 builds it.
func checkDrift(context.Context, *run) error { return errNotBuilt }
