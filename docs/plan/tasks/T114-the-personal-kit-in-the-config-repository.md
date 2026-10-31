---
id: T114
title: "The personal kit in the config repository"
milestone: M2
module: none
fate: new
owns: []
needs:
  - T079
after: []
ask_first: []
sitting: M2
---

# T114 - The personal kit in the config repository

Goal: One personal kit in the config repository: the instructions file,
the settings merge that never drops hooks or plugins (06 6.5), the
pinned plugin marketplaces, the status line and themes, the
context-usage tools, in Go where it fits.

No file of this repository: the work lands in the maintainer's config
repository, and this file is its record and its checkbox. The kit and
its plugin pins are an ask-first change there, under that repository's
own rules, since they enter every sandbox (rule 9 of `docs/plan.md`).
The pull request whose Evidence carries sitting M2's output names this
task in its body and writes `merged:` and `pr:` into this file, the one
write rule 1 allows outside its own file.

- Implements: REQ-0608 (06 6.5).
- Operator: builds a daily sandbox with it.
- Acceptance:
  - A daily sandbox built with it and `julieta-claude` delivers the
    instructions file, the merged settings, the plugins at their pins,
    the status line and themes, and the context-usage tools.
  - The settings merge keeps the hooks and the plugins of
    `julieta-claude` (06 6.5).
  - It carries the skill-router hook that the interim payload it
    replaces gained on 2026-10-31: the line that points every session
    and subagent at `agent-skills:using-agent-skills` (SessionStart,
    SubagentStart) and the local usage log of starts and Skill calls
    (PostToolUse on `Skill`), with its `--report`.
- Verify: one daily sandbox, read at sitting M2: its SessionStart output
  shows the instructions and the status line, `julieta status --json`
  reports no `agent-hooks-missing`, and the plugin list shows the pinned
  versions; a test push on a throwaway branch from that sandbox; the
  hook output names both hooks.
