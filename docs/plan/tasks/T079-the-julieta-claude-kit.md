---
id: T079
title: "The `julieta-claude` kit"
milestone: M2
module: kits
fate: kept
owns:
  - kits/julieta-claude/**
  - internal/agent/claude/**
  - internal/cmd/julieta/setup/skills_stale.go
  - internal/cmd/julieta/status/agent_hooks.go
  - e2e/kit_julieta_claude_test.go
  - internal/cmd/julieta/setup/agent_paths.go
  - internal/cmd/julieta/hooks/guard*.go
needs:
  - T077
  - T078
after: []
ask_first:
  - kits
sitting: none
---

# T079 - The `julieta-claude` kit

Goal: The `julieta-claude` kit: the two hooks for every source as
wrappers on the `.romeu/bin` path, the PreToolUse guard that fails
closed, the skills, the agent memory directory made non-writable;
`setup`'s `skills-stale` and `status`'s `agent-hooks-missing` as one
file each in those packages; `julieta hooks guard`.

Re-plan (2026-10-12): Kept. Acceptance added: it replaces the hook lines
of the interim kit (T111) in a daily sandbox; each refusal has a
container e2e case, in `e2e/kit_julieta_claude_test.go` (rule 1 of
`docs/plan.md`). The hook command takes julieta's path from the
`julietaBin` arg (06 6.4): until sitting M3 the arg is set in the
project definition by hand, the fourth edit of rule 9 (T115), and `romeu
sync` writes it afterwards. The agent profile of `internal/agent/claude`
declares the process names and the required state paths and no pane; the
open-session record `agent-not-matched` reads (T061) comes from the
SessionStart and SessionEnd hooks this kit wires. `setup/steps.go` is
the seam it extends by one line (rule 14).

- Implements: REQ-0008 (index), REQ-0010 (index), REQ-0114 (02 2.1),
  REQ-0402 (04 4.3), REQ-0410 (04 4.3), REQ-0424 (04 4.3), REQ-0547 (05
  5.4), REQ-0587 (06 6.3), REQ-0595 (06 6.4), REQ-0596 (06 6.4),
  REQ-0597 (06 6.4), REQ-0598 (06 6.4), REQ-0599 (06 6.4), REQ-0607 (06
  6.4), REQ-0608 (06 6.5), REQ-0658 (08 8.1), REQ-0663 (08 8.2),
  REQ-0666 (08 8.2), REQ-0668 (08 8.2), REQ-0671 (08 8.2), REQ-0672 (08
  8.2), REQ-0673 (08 8.2), REQ-0707 (08 8.5), REQ-0741 (10 10.1),
  REQ-0744 (10 10.1), REQ-0745 (10 10.1), REQ-0746 (10 10.1).
- Operator: no.
- Acceptance:
  - The kit installs the two skills of T078 and the SessionStart and
    SessionEnd hooks, and makes the agent's own memory directory
    non-writable (Q8).
  - One container e2e case per refusal of 06 6.4 (`gh pr merge`, a `v*`
    tag push, `gh api` to rulesets and to repository settings), each
    asserting the refusal text says the action is the maintainer's and
    names the route (an issue, or the handoff).
  - `julieta hooks guard` reads one PreToolUse document on stdin and
    exits 2 on the three patterns and on an undecodable document; the
    container e2e feeds the recorded fixture, one case per pattern and
    one for an undecodable document (PROVISIONAL: item 14 of Decisions
    open at the re-plan in `docs/plan.md`, recommended keep).
  - The rendered hook config wires SessionStart for `startup`, `resume`,
    `clear` and `compact`, and SessionEnd once.
  - With the `.romeu/bin` mount absent, each session-hook wrapper prints
    the failure line and exits 0, and the PreToolUse wrapper exits 2:
    the case titled "PreToolUse with the .romeu/bin mount absent refuses
    gh pr merge" passes.
  - julieta embeds the digest of the skills and hook lines this kit
    installs; `setup` reports `skills-stale` when the installed ones
    differ, and SessionStart prints it (06 6.3 step 6).
  - The claude profile lists its paths as required or optional; `julieta
    setup` warns once per missing required path and SessionStart shows
    the warning (08 8.2 item 1).
  - It is the one agent-aware kit: a test fails an agent-specific path
    in `kits/julieta/`.
  - That the hooks and the skills are present in a real sandbox is
    measured by B2 (S5).
- Verify: `go run ./tools/ci kits`; `go test -tags e2e ./e2e/... -run
  '^TestKitJulietaClaude' -v`; `go test ./internal/agent/claude/...`; in
  the daily sandbox where it replaced the interim kit's hook lines, a
  test push on a throwaway branch from that sandbox; the hook output
  names both hooks.
