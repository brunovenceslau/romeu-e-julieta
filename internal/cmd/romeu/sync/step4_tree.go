// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import "context"

// makeTree is step 4 of 04 4.2: create <name>-env/ and its memory dirs,
// clone the missing repos, fetch each origin and resolve the egress
// commits (REQ-0339). Lane B of T043 builds it.
func makeTree(context.Context, *run) error { return errNotBuilt }
