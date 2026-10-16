// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

import "os/exec"

// LookPath returns the path of the program that Run starts for a bare
// name, looked up in this process's PATH as Run looks it up; a name
// that holds a slash is checked as it is. A program that is not there
// is ErrNotFound. It is os/exec's LookPath, here so that a caller, a
// test of the tools among them, finds a program without importing
// os/exec, which the call-site scan holds to this package.
func LookPath(name string) (string, error) {
	return exec.LookPath(name)
}
