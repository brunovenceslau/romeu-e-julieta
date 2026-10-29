// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import "context"

// readSpecs is step 3 of 04 4.2: git fsck --strict on the spec commit,
// then the strict decode and validation of every projects/*.yaml,
// listing every error (REQ-0337, REQ-0338). Lane A of T043 builds it.
func readSpecs(context.Context, *run) error { return errNotBuilt }
