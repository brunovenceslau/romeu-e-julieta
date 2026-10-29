// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import "context"

// lockAndCheck is step 1 of 04 4.2: take the lock, which runs promotion
// recovery; load and validate the host settings; check gate 1
// (REQ-0335). Recovery runs on every <name>-env/ under the root, since
// the targets are known only at step 3. Lane A of T043 builds it.
func lockAndCheck(context.Context, *run) error { return errNotBuilt }
