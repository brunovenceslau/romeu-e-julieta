// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

// SetAgentManaged makes setup read the agent's managed settings below
// dir until the returned function restores the directory.
func SetAgentManaged(dir string) (restore func()) {
	prev := agentManaged
	agentManaged = dir
	return func() { agentManaged = prev }
}
