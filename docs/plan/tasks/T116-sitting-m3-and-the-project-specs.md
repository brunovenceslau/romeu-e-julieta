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

Amended on 2026-11-07 (the sbx 0.51 and kit schema m.12 amendments,
as decided on 2026-11-07): the `dockerd@1` row of Deferred decisions can
fire at the first slot whose project cannot run its checks without a
Docker engine in the sandbox (this repository is named), and is decided
from that slot's hand record of what A19 asks, which A19 re-records in
M6. The env file cannot turn off the host's skills store, so a romeu
sandbox gets it read-only while sbx's `skills.defaultMode` is `readonly`
(03 3.3, REQ-0200; `doctor`'s `skills-shared`; amended on 2026-11-07);
each slot records the setting in force and checks the skills the
project's sessions use against T114's list, so no slot depends on the
share.

- Implements: REQ-1091 (12 12.2).
- Operator: runs the slots; from a `--dry-run` build of `main` (T103).
  Each sitting builds from a commit whose diff since the last host build
  the maintainer has read, named with its SHA in the sitting's Evidence.
- Acceptance:
  - Every project definition is rendered by `romeu sync`.
  - Each runbook section holds the slot's undo.
  - Each slot records whether the project's checks run in its sandbox
    (for this repository, a Docker engine for the hybrid and container
    e2e). A slot that cannot run them opens the `deferred` issue of the
    `dockerd@1` row before the next slot and records by hand, in that
    issue, what A19 asks for the workload in force: the engine's uid, its
    socket's group and what the agent user reaches.
  - Each slot records the `skills.defaultMode` in force and whether a
    skill its sessions use is missing from the rendered sandbox's kits; a missing one is added to T114's kit or
    reported as the trigger of the `skills` row before the next slot.
  - Every issue opened here from an item of the retired capabilities
    passes `go run ./tools/ci hygiene --file <body>` before it is
    opened, and the runbook lists the item ids moved.
  - The transcript of each slot's commands and its `romeu status
    --json`, read by the maintainer, are the Evidence of the next pull
    request (the smoke steps of 12 12.2, applied per project); the host
    suite (T075) is M6 and no slot runs it.
- Verify: `romeu status --json` exits 0 on each project and shows a
  generation rendered by `sync`, read at the sitting.
