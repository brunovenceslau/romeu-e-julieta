// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage

// SetAfterCapture sets the seam of the recheck of step 2 for the test
// and restores it when the test ends.
func SetAfterCapture(t interface{ Cleanup(func()) }, f func(dir string)) {
	old := afterCapture
	afterCapture = f
	t.Cleanup(func() { afterCapture = old })
}

// FailReason is failReason.
var FailReason = failReason
