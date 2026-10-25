// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package status

// SetAgent makes the section of the agent's hooks read the managed
// settings below managed for julieta at self, until the returned
// function restores both.
func SetAgent(managed, self string) (restore func()) {
	prevManaged, prevExec := agentManaged, executable
	agentManaged = managed
	executable = func() (string, error) { return self, nil }
	return func() { agentManaged, executable = prevManaged, prevExec }
}
