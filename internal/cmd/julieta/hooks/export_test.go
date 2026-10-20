// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks

import "io"

// Process replaces the stdin, the environment and the exit of the
// command and returns the function that restores them.
func Process(in io.Reader, env []string, exitFn func(int)) func() {
	oldIn, oldEnv, oldExit := stdin, environ, exit
	stdin, environ, exit = in, func() []string { return env }, exitFn
	return func() { stdin, environ, exit = oldIn, oldEnv, oldExit }
}
