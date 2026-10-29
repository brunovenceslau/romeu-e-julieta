// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import "context"

// gateAndPromote is step 8 of 04 4.2: gate 2 and the candidate state
// machine; the promotion commit on approval or when unchanged
// (REQ-0343). Lane D of T043 builds it.
func gateAndPromote(context.Context, *run) error { return errNotBuilt }
