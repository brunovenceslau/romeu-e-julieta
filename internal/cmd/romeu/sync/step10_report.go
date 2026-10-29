// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import "context"

// report is step 10 of 04 4.2: report orphaned projects and repo dirs,
// removing nothing, and warn julieta-pin-mismatch (REQ-0345). Lane E of
// T043 builds it.
func report(context.Context, *run) error { return errNotBuilt }
