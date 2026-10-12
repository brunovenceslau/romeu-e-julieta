---
id: T119
title: "Sitting M5: the closing read"
milestone: M5
module: none
fate: new
owns: []
needs: []
after:
  - T116
ask_first: []
sitting: M5
---

# T119 - Sitting M5: the closing read

Goal: The read that ends M5: one month of daily use after the last slot
of sitting M3 with no hand-applied step and no open runbook section;
every open item the maintainer tracks outside julieta becomes a
`julieta memory` entry of its project or closes, in one recorded list.

No file of this repository: the work lands in the maintainer's config
repository, and this file is its record and its checkbox.

Re-plan (2026-10-12): After sitting M3 (`after: T116`), since the month
of daily use starts at its last slot.

- Implements: 08 8.1 (memory as the one place for open items).
- Operator: reads the list at sitting M5.
- Acceptance:
  - One list, each item with its destination: a memory entry id, or
    closed.
  - No runbook section is open, and every project's `romeu status
    --json` shows a generation rendered by `sync`.
- Verify: read at sitting M5: the list; `romeu status --json` exits 0
  on each project; `julieta memory list` in each project's sandbox
  shows the entries the list names.
