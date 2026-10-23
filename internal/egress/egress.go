// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package egress derives the domains a sandbox may reach from the
// mise.lock of each of its repos, at the commit romeu pinned (07 7.5,
// REQ-0057). It runs no mise and reads no work tree: a lock is data
// (I9). 10 10.7 keeps it from importing internal/mise, a package of
// julieta, so it reads the lock with a reader of its own (ParseLock).
package egress
