---
id: T078
title: "The skills"
milestone: M1
module: skills
fate: kept
owns:
  - skills/**
  - internal/cli/skills_test.go
needs:
  - T059
after:
  - T107
ask_first:
  - kits
sitting: none
---

# T078 - The skills

Goal: `skills/julieta/SKILL.md`, `skills/handoff/SKILL.md`, and the test
that every command a skill names is registered.

Re-plan (2026-10-12): Kept, in M1: sitting M1 needs the skills.
`skills/**` joins the `kits` surface in T107.

- Implements: 02 2.1 (`skills/`), 08 8.1, 08 8.3.
- Operator: no.
- Acceptance:
  - `skills/julieta/SKILL.md` tells the agent to use `julieta memory`
    and to tag a lesson; `skills/handoff/SKILL.md` pipes the required
    headings to `julieta handoff write [--final]`.
  - A test checks that each `julieta` command a skill names exists in
    the command definitions.
- Verify: `go test ./... -run '^TestSkills'`, which lists that test by
  name.
