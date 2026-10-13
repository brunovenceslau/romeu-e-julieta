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
every open item of a project is a `julieta memory` entry or closed, in
one recorded list; the entry of the store's items ran at sitting M1
(T111), and this read lists what is left.

No file of this repository: the work lands in the maintainer's config
repository, and this file is its record and its checkbox. The pull
request whose Evidence carries the sitting's output names this task in
its body and writes `merged:` and `pr:` into this file, the one write
rule 1 allows outside its own file.

Re-plan (2026-10-12): After sitting M3 and sitting M4: the month of
daily use starts at the later of the two. The cut-over of the store's
items is T111's (item 19 of Decisions open at the re-plan); this task
reads the result.

- Implements: none (a sitting: its measured fact is the M5 row of
  `docs/plan.md`, and no obligation of the specification names it).
- Operator: reads the list at sitting M5.
  Each sitting builds from a commit whose diff since the last host build
  the maintainer has read, named with its SHA in the sitting's Evidence.
- Acceptance:
  - One list, each item with its destination: a memory entry id, or
    closed.
  - No runbook section is open, and every project's `romeu status
    --json` shows a generation rendered by `sync`.
- Verify: read at sitting M5: the list; `romeu status --json` exits 0 on
  each project; `julieta memory list` in each project's sandbox shows
  the entries the list names.
