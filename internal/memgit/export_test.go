// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package memgit

// SetMaxTotal sets the bound of the bytes Sync stages for a test, and
// returns the function that restores it.
func SetMaxTotal(n int64) func() {
	old := maxTotal
	maxTotal = n
	return func() { maxTotal = old }
}
