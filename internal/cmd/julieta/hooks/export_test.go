// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks

import (
	"io"
	"strings"
)

// Process replaces the stdin, the environment and the exit of the
// command and returns the function that restores them. Each run reads
// in from a reader of its own, as each process has its own stdin.
func Process(in string, env []string, exitFn func(int)) func() {
	oldIn, oldEnv, oldExit := stdin, environ, exit
	stdin = func() io.Reader { return strings.NewReader(in) }
	environ, exit = func() []string { return env }, exitFn
	return func() { stdin, environ, exit = oldIn, oldEnv, oldExit }
}
