// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package status

import (
	"context"
	"os"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/agent/claude"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
)

// The agent's managed settings directory and the running binary, read
// by the section below; a test replaces them.
var (
	agentManaged = claude.ManagedDir
	executable   = os.Executable
)

// The section of the agent's hooks and skills (04 4.3, REQ-0424; 06
// 6.5, REQ-0608): whether the agent's settings carry the hooks with
// julieta's exact commands, for the running julieta, and both skills
// are installed, read live, so a personal kit that removed one shows.
// agent-hooks-missing takes the place of the one the last setup
// recorded, which may be stale, among the warnings the warnings section
// prints; a settings file that cannot be read is one entry of errors.
// A manifest whose agent handle no profile serves reads nothing.
var _ = register(section{
	name:  "agent-hooks",
	order: 91,
	fill: func(_ context.Context, src *source, rep *Report) {
		if !claude.Matches(src.m.Agent) {
			return
		}
		self, err := executable()
		if err != nil {
			rep.Errors = append(rep.Errors, "agent hooks: the running binary cannot be found: "+err.Error())
			return
		}
		w, missing, err := setup.AgentHooks(src.home, agentManaged, self)
		if err != nil {
			rep.Errors = append(rep.Errors, "agent hooks: "+err.Error())
			return
		}
		rep.Warnings = slices.DeleteFunc(rep.Warnings, func(w setup.Warning) bool { return w.Slug == setup.AgentHooksMissing })
		if missing {
			rep.Warnings = append(rep.Warnings, w)
		}
	},
	text: func(_, _ *cli.Writer, _ *Report) error { return nil },
})
