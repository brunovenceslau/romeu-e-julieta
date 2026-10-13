---
id: T123
title: "The records of the product decisions and of review round 8"
milestone: M1
module: docs
fate: new
owns:
  - docs/adr/0014-*.md
  - docs/adr/0015-*.md
  - docs/adr/0016-*.md
  - docs/adr/0017-*.md
  - docs/adr/0018-*.md
  - docs/adr/0019-*.md
  - docs/adr/0020-*.md
needs:
  - T106
after:
  - T108
ask_first:
  - decisions
sitting: none
---

# T123 - The records of the product decisions and of review round 8

Goal: One record per row of the product-decision table of 12 12.5 (a
secondary repo cloned by julieta; the two gates; v3 kits only; herdr as
the one renderer; memory as plain per-repo files; stdlib `flag`;
GPL-3.0-only with CC0-1.0 for `examples/` and `schemas/`) and one per
decision of round 8 that ADR 0009 does not hold, each numbered by
`tools/new adr`. The `owns` globs are provisional: the writer replaces
them by the numbers `tools/new adr` gives, in order. A record is a
review order: T035, T040 and T077 come `after` this task, and no task
`needs` it.

Re-plan (2026-10-12, stage B): New. 12 12.5 asks that a decision the
specification cites as authority has its record in or before the first
task that implements the citing section.

- Implements: REQ-1169 (12 12.5), REQ-1170 (12 12.5), REQ-1171 (12
  12.5).
- Operator: no
- Acceptance:
  - Each row of the table names its record and leaves the table.
  - `tools/ci generated` passes after the ADR index regenerates.
  - T035, T040 and T077 cite "the record of <decision> is merged (12
    12.5)".
- Verify: `go run ./tools/ci generated` exits 0; `go run ./tools/ci
  sequences` once T004 lands.
