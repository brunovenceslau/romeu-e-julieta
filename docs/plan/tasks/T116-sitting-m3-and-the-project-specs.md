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
`julieta snapshot --all`, `sbx env rm` of the sandbox the hand-written
definition started (the preflight refuses a sandbox romeu did not
record, 01 1.5 step 6, and `adopt` is deferred), `romeu sync`, gate 2
read, `romeu run` into a fresh sandbox, the hand-written definition
deleted; a runbook section per project in the config repository with the
undo of the slot: restore the definition from the repository's history
and start it again, which reads the same memory directory, since the
bridge mounted it at its final host path (02 2.3). The slots of one
sitting are one runbook pull request.

No file of this repository: the work lands in the maintainer's config
repository, and this file is its record and its checkbox. The pull
request whose Evidence carries the sitting's output names this task in
its body and writes `merged:` and `pr:` into this file, the one write
rule 1 allows outside its own file.

- Implements: REQ-1091 (12 12.2).
- Operator: runs the slots; from a `--dry-run` build of `main` (T103).
  Each sitting builds from a commit whose diff since the last host build
  the maintainer has read, named with its SHA in the sitting's Evidence.
- Acceptance:
  - Every project definition is rendered by `romeu sync`.
  - Each runbook section holds the slot's undo.
  - Every issue opened here from an item of the retired capabilities
    passes `go run ./tools/ci hygiene --file <body>` before it is
    opened, and the runbook lists the item ids moved.
  - The transcript of each slot's commands and its `romeu status
    --json`, read by the maintainer, are the Evidence of the next pull
    request (the smoke steps of 12 12.2, applied per project); the host
    suite (T075) is M6 and no slot runs it.
- Verify: `romeu status --json` exits 0 on each project and shows a
  generation rendered by `sync`, read at the sitting.
