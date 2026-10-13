---
id: T119
title: "Sitting M5: the closing read"
milestone: M5
module: none
fate: new
owns: []
needs:
  - T116
  - T117
after: []
ask_first: []
sitting: M5
---

# T119 - Sitting M5: the closing read

Goal: The read that ends M5: one month of daily use after the last slot
of sitting M3 with no hand-applied step and no open runbook section;
every open item of a project is still a `julieta memory` entry or
closed: the store holds no item that the open-items list of sitting M1
(T111) does not name.

No file of this repository: the work lands in the maintainer's config
repository, and this file is its record and its checkbox. The pull
request whose Evidence carries the sitting's output names this task in
its body and writes `merged:` and `pr:` into this file, the one write
rule 1 allows outside its own file.

Re-plan (2026-10-12): After sitting M3 and sitting M4: the month of
daily use starts at the later of the two. The cut-over of the store's
items is T111's, at sitting M1 (item 19 of Decisions of the re-plan,
amended on 2026-10-13); this task reads that it holds.

- Implements: none (a sitting: its measured fact is the M5 row of
  `docs/plan.md`, and no obligation of the specification names it).
- Operator: reads the store and the open-items list of T111 at sitting
  M5.
  Each sitting builds from a commit whose diff since the last host build
  the maintainer has read, named with its SHA in the sitting's Evidence.
- Acceptance:
  - The store holds no item that the open-items list of sitting M1
    does not name, each named item with its destination: a memory
    entry id, or closed.
  - No runbook section is open, and every project's `romeu status
    --json` shows a generation rendered by `sync`.
- Verify: read at sitting M5: the store against the open-items list of
  sitting M1; `romeu status --json` exits 0 on each project; `julieta
  memory list` in each project's sandbox shows the entries the list
  names.
