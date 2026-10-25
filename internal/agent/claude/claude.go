// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package claude is the agent profile of Claude Code, the only
// Claude-aware code (02 2.1, REQ-0114): the process names and state
// paths salvage and setup read (08 8.5 step 5), the hook commands the
// julieta-claude kit wires (06 6.4, 08 8.2), the reads of the agent's
// settings and skills that julieta setup and julieta status make, the
// digest of what the kit installs (06 6.3, REQ-0587), and the decoder
// of the document a PreToolUse hook gets. The profile has no pane.
package claude

import "slices"

// Name is the profile's name.
const Name = "claude"

// handles are the agent handles of the manifest (03 3.6) this profile
// serves: sbx's built-in agent and the workload kit of the pin file.
var handles = []string{"claude", "sbx-kit-claude"}

// Matches reports whether the agent handle of a manifest names this
// profile.
func Matches(agent string) bool { return slices.Contains(handles, agent) }

// Processes are the names of the agent's processes, which salvage
// stops (08 8.5 step 1). Probe C5 confirms them for the pinned version.
var Processes = []string{"claude"}

// Path is one state path of the agent, below $HOME.
type Path struct {
	// Rel is the path relative to $HOME, clean.
	Rel string
	// Required says that the path exists at the pinned agent, so a
	// missing one is agent-path-missing (08 8.5 step 5); an optional
	// one, which a session may never create, is no finding.
	Required bool
	// What says what the path holds.
	What string
}

// Paths are the agent's state paths. The required ones are the
// workload's volumes, which exist from the sandbox's create on.
var Paths = []Path{
	{Rel: ".claude/projects", Required: true, What: "the transcripts and the state of each project"},
	{Rel: ".claude/todos", Required: true, What: "the todo lists of the sessions"},
	{Rel: ".claude/plans", What: "the plans the sessions wrote"},
}

// MemoryDir is the agent's own memory dir: the kit points the setting
// autoMemoryDirectory at it and makes it empty and not writable (08
// 8.1, Q8). It is outside $HOME, so the agent cannot remove it.
const MemoryDir = "/var/lib/julieta-claude/agent-memory"
