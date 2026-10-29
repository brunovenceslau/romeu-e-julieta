// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import "context"

// fetchConfig is step 2 of 04 4.2: the hardened fetch of the config
// repo's origin into refs/romeu/origin/<cfg-dir>/*, and the spec commit
// (REQ-0336). Lane A of T043 builds it.
func fetchConfig(context.Context, *run) error { return errNotBuilt }
