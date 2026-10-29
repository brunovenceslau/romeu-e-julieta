// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import "context"

// renderCandidate is step 6 of 04 4.2: read the descriptors, cache
// first; derive egress; delete the candidate dirs the record does not
// name; run the render-time rules; render the candidate (REQ-0341).
// Lane C of T043 builds it.
func renderCandidate(context.Context, *run) error { return errNotBuilt }
