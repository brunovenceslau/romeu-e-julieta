// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"context"
	"fmt"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/agent/claude"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// The warnings of the skills and hooks the julieta-claude kit installs
// (04 4.1, Warnings).
const (
	// SkillsStale is skills-stale: the installed skills and hook lines
	// differ from the digest julieta embeds (06 6.3, REQ-0587).
	SkillsStale = "skills-stale"
	// AgentHooksMissing is agent-hooks-missing: the agent's settings
	// lack one of julieta's exact hook commands, or a skill is not
	// installed (04 4.3, REQ-0424).
	AgentHooksMissing = "agent-hooks-missing"
)

// agentManaged is the directory of the agent's managed settings; a
// test replaces it.
var agentManaged = claude.ManagedDir

// checkSkillsStale is the step of setup that compares the skills and
// hook lines installed with the digest julieta embeds (04 4.3,
// REQ-0402): skills-stale when they differ, which the next SessionStart
// prints (06 6.3 step 6), and agent-hooks-missing when a hook or a
// skill is missing (REQ-0424). The hook lines are read for the running
// julieta, so a kit installed for another julieta is stale. A handle
// that no profile serves checks nothing.
func checkSkillsStale(_ context.Context, r *run) {
	if !claude.Matches(r.m.Agent) {
		return
	}
	self, err := executable()
	if err != nil {
		r.report.Errors = append(r.report.Errors, "skills-stale: the running binary cannot be found: "+termsafe.Escape(err.Error()))
		return
	}
	dropIn := claude.DropInAt(agentManaged)
	files, err := claude.Installed(r.home, dropIn, self)
	switch {
	case err != nil:
		r.warn(SkillsStale, "the installed skills and hook lines cannot be read: "+termsafe.Escape(err.Error()))
	case claude.Digest(files) != claude.KitDigest:
		r.warn(SkillsStale, fmt.Sprintf("the skills in %s and the hook lines in %s are not those of the julieta-claude kit of this julieta (%s, not %s); a recreate installs them",
			termsafe.Escape(r.home+"/"+claude.SkillsDir), termsafe.Escape(dropIn), claude.Digest(files), claude.KitDigest))
	}
	if w, ok, err := AgentHooks(r.home, agentManaged, self); err != nil {
		r.report.Errors = append(r.report.Errors, "agent-hooks-missing: "+termsafe.Escape(err.Error()))
	} else if ok {
		r.warn(w.Slug, w.Message)
	}
}

// AgentHooks returns the warning agent-hooks-missing, and true, when
// the agent's settings files (below managed and home) lack one of the
// hook commands julieta at the path julieta wires, or a skill of the
// kit is not installed (04 4.3, REQ-0424); julieta setup records it and
// julieta status reads it live. A settings file that cannot be read is
// an error.
func AgentHooks(home, managed, julieta string) (Warning, bool, error) {
	files, err := claude.SettingsFiles(managed, home)
	if err != nil {
		return Warning{}, false, err
	}
	missing, err := claude.Missing(home, files, julieta)
	if err != nil || len(missing) == 0 {
		return Warning{}, false, err
	}
	return Warning{Slug: AgentHooksMissing, Message: fmt.Sprintf("the agent's settings and skills lack julieta's %s; the julieta-claude kit installs them, and a personal kit must not remove them",
		termsafe.Escape(strings.Join(missing, ", ")))}, true, nil
}
