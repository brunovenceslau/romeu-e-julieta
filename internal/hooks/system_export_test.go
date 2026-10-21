// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks

// SetMaxStdin sets the bound of the stdin the dispatcher reads and
// returns the function that restores it.
func SetMaxStdin(n int) (restore func()) {
	old := maxStdin
	maxStdin = n
	return func() { maxStdin = old }
}
