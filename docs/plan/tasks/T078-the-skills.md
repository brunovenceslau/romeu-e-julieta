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

- Implements: REQ-0544 (05 5.3), REQ-0604 (06 6.4), REQ-0660 (08 8.1),
  REQ-0662 (08 8.1), REQ-0676 (08 8.3), REQ-0852 (10 10.2), REQ-1115 (12
  12.3), REQ-1255 (12 12.3).
- Operator: no.
- Acceptance:
  - `skills/julieta/SKILL.md` tells the agent to use `julieta memory`
    and to tag a lesson; the handoff skill names the required headings
    of each kind in its references, and `skills/handoff/SKILL.md` pipes
    the narrative to `julieta handoff write [--final]`.
  - `skills/julieta/SKILL.md` tells the agent, when signing a commit
    fails or waits on a confirmation that does not come, to ask the
    operator and never to unset `commit.gpgsign` or sign another way
    **[review]**.
  - One test fails when a julieta command or flag named in a SKILL.md is
    not in the command definitions, or when the headings
    `skills/handoff/SKILL.md` names for a kind differ from those
    `julieta handoff write` requires for it; fixtures: an unknown flag,
    a missing heading, an extra heading (12 12.3).
  - Each skill follows REQ-1255: a description of what it does and
    when to use it, a body under 500 lines, and its other files linked
    from `SKILL.md`; the same test fails a reference no line links and
    a link to no file.
- Verify: `go test ./... -run '^TestSkills'`, which lists that test by
  name.
