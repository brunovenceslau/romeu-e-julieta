// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import "os"

// SetExecutable makes f find the running binary until the returned
// function restores os.Executable.
func SetExecutable(f func() (string, error)) (restore func()) {
	executable = f
	return func() { executable = os.Executable }
}
