---
id: T116
title: "Sitting M3 and the project specs"
milestone: M3
module: none
fate: new
owns: []
needs:
  - T066
  - T055
  - T080
  - T114
after: []
ask_first: []
sitting: M3
---

# T116 - Sitting M3 and the project specs

Goal: The config project's `projects/<name>.yaml` first, this repository
second, then one project per slot; each slot: `/handoff --final`, push,
`julieta snapshot --all`, `romeu sync`, gate 2 read, `romeu run` into a
fresh sandbox, the hand-written definition deleted; a runbook section
per project in the config repository.

No file of this repository: the work lands in the maintainer's config
repository, and this file is its record and its checkbox.

- Implements: 09 (J1, J2, J3); 12 12.2 (the smoke steps, one per project
  slot).
- Operator: runs the slots; from a `--dry-run` build of `main` (T103).
- Acceptance:
  - Every project definition is rendered by `romeu sync`.
  - The transcript of each slot's commands and its `romeu status
    --json`, read by the maintainer, are the Evidence of the next pull
    request (amendment 7 in `docs/plan.md`); the host suite (T075) is
    M6 and no slot runs it.
- Verify: `romeu status --json` exits 0 on each project and shows a
  generation rendered by `sync`, read at the sitting.
