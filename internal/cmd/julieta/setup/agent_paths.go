// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/internal/agent/claude"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// AgentPathMissing is the warning agent-path-missing (04 4.1,
// Warnings): a required path of the agent profile is absent, so the
// profile no longer matches the pinned agent (08 8.5 step 5).
const AgentPathMissing = "agent-path-missing"

// profileMemoryDir returns the agent's own memory dir of the profile
// the manifest's agent handle names, and false when no profile of this
// build serves the handle.
func profileMemoryDir(_, agent string) (string, bool) {
	if claude.Matches(agent) {
		return claude.MemoryDir, true
	}
	return "", false
}

// checkAgentPaths is the step of setup that checks the required paths
// of the agent profile (04 4.3, REQ-0401): one warning
// agent-path-missing for each required path that is not below $HOME,
// which julieta status and the next SessionStart show from julieta
// state (08 8.2). An optional path is not checked, and a handle that no
// profile serves checks nothing.
func checkAgentPaths(_ context.Context, r *run) {
	if !claude.Matches(r.m.Agent) {
		return
	}
	for _, p := range claude.Paths {
		if !p.Required {
			continue
		}
		path := filepath.Join(r.home, p.Rel)
		if _, err := os.Stat(path); err != nil {
			r.warn(AgentPathMissing, fmt.Sprintf("%s, %s, is missing for the agent profile %s: %s",
				termsafe.Escape(path), p.What, claude.Name, termsafe.Escape(err.Error())))
		}
	}
}
