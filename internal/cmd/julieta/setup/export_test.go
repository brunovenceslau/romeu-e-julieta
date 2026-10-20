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

// WriteState exposes writeState to the external tests.
var WriteState = writeState

// SetStateOnWait sets the seam called when the lock of setup.json is
// held by another and returns the function that restores it.
func SetStateOnWait(f func()) (restore func()) {
	old := stateOnWait
	stateOnWait = f
	return func() { stateOnWait = old }
}

// RecordState exposes recordState to the external tests.
var RecordState = recordState
