// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

// SetAgentMemoryDir makes the agent memory dir check read the dir of f
// until the returned function restores the lookup.
func SetAgentMemoryDir(f func(home, agent string) (string, bool)) (restore func()) {
	prev := agentMemoryDir
	agentMemoryDir = f
	return func() { agentMemoryDir = prev }
}
