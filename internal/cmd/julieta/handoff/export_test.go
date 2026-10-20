// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff

// SetOnWait sets the seam called when a lock is held by another
// process and returns the function that restores it.
func SetOnWait(f func(lock string)) (restore func()) {
	old := onWait
	onWait = f
	return func() { onWait = old }
}

// SetBeforeGit sets the seam called before the first git read of a
// write, with the lock held, and returns the function that restores it.
func SetBeforeGit(f func()) (restore func()) {
	old := beforeGit
	beforeGit = f
	return func() { beforeGit = old }
}
