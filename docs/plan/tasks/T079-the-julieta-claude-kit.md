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
file each in those packages.

Re-plan (2026-10-12): Kept. Acceptance added: it replaces the hook lines
of the interim kit (T111) in a daily sandbox; each refusal has a
container e2e case, in `e2e/kit_julieta_claude_test.go` (rule 1). The
hook command takes julieta's path from the `julietaBin` arg (06 6.4):
until sitting M3 the arg is set in the project definition by hand, the
fourth edit of rule 9 (T115), and `romeu sync` writes it afterwards.

- Implements: 06 6.4, 08 8.1 ("How julieta replaces the agent's built-in
  memory"), 08 8.2, S5.
- Operator: no.
- Acceptance:
  - The kit installs the two skills of T078 and the SessionStart and
    SessionEnd hooks, and makes the agent's own memory directory
    non-writable (Q8).
  - It is the one agent-aware kit: a test fails an agent-specific path
    in `kits/julieta/`.
  - That the hooks and the skills are present in a real sandbox is
    measured by B2 (S5).
- Verify: `go run ./tools/ci kits`; `go test -tags e2e ./e2e/... -run
  '^TestKitJulietaClaude' -v`; `go test ./internal/agent/claude/...`;
  in the daily sandbox where it replaced the interim kit's hook lines,
  a test push on a throwaway branch from that sandbox; the hook output
  names both hooks.
