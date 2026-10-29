// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import "context"

// checkMemory is step 5 of 04 4.2: refuse a memory dir that breaks the
// layout allowlist, I24 (REQ-0340). Lane B of T043 builds it.
func checkMemory(context.Context, *run) error { return errNotBuilt }
