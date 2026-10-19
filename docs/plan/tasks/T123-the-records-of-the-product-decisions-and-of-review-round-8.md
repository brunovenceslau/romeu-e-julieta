---
id: T123
title: "The records of the product decisions and of review round 8"
milestone: M1
module: docs
fate: merged
owns:
  - docs/adr/0017-*.md
  - docs/adr/0018-*.md
  - docs/adr/0019-*.md
  - docs/adr/0020-*.md
  - docs/adr/0021-*.md
  - docs/adr/0022-*.md
  - docs/adr/0023-*.md
  - docs/adr/0024-*.md
  - docs/adr/0025-*.md
  - docs/adr/0026-*.md
  - docs/adr/0027-*.md
  - docs/adr/0028-*.md
  - docs/adr/0029-*.md
  - docs/adr/0030-*.md
  - docs/adr/0031-*.md
  - docs/adr/0032-*.md
  - docs/adr/0033-*.md
  - docs/adr/0034-*.md
  - docs/adr/0035-*.md
  - docs/spec/12-engineering.md
  - docs/spec/00-scope.md
  - docs/plan/tasks/T035-*.md
  - docs/plan/tasks/T040-*.md
  - docs/plan/tasks/T077-*.md
needs: []
after: []
ask_first:
  - decisions
sitting: none
merged: 2026-10-18
pr: 155
---

# T123 - The records of the product decisions and of review round 8

Merged on 2026-10-18 in pull request #155. The text below is the one
written at the re-plan.

Goal: One record per row of the product-decision table of 12 12.5 (a
secondary repo cloned by julieta; the two gates; v3 kits only; herdr as
the one renderer; memory as plain per-repo files;
GPL-3.0-only with CC0-1.0 for `examples/` and `schemas/`) and one per
decision of round 8 chosen by the rule of the re-plan below and not held by
[ADR 0009, drop the required review and amend the records that assumed it](../../adr/0009-drop-the-required-review-and-amend-the-records-that-assumed-it.md),
each numbered by
`tools/new adr`. The `owns` globs are provisional: the writer replaces
them by the numbers `tools/new adr` gives, in order, and the collision
rule of 12 12.3 covers a number taken twice, so this task waits for no
M0 task. A record is a review order: T035, T040 and T077 come `after`
this task, and no task `needs` it.

Re-plan (2026-10-18): Changed. The `owns` globs name the real records:
nineteen, 0017 to 0035. Six are for the rows of the table of 12 12.5.
Thirteen are for decisions of round 8 that ADR 0009 does not hold,
chosen by the rule of the specification: a settled question becomes a
record when the decision is expensive to reverse (REQ-1168), and a
decision the specification cites as authority gets one (REQ-1169). In
practice a round-8 item gets a record when it reverses or narrows an
earlier decision, or changes the scope or the plan; an accepted risk
applied as specification text gets none. The chosen items are SC1
(0023), SC2 (0024), SC3 (0028), SC4 (0029), SC5 (0030), SC10 (0031),
SC12 (0025), SC14 (0032), SC15 (0033), SC16 (0034), AR10 (0035), AR13
(0026) and AR14 (0027). The operator confirms the selection in the
pull request.

Skipped, each with its reason:

- SC6, SC8 and SC9 keep what exists (`handoff list`, gate 1, `pin`),
  and the gate 1 sentence is in the record of the two gates.
- SC7 only rewords the v1.1 bucket as Deferred rows.
- SC11 adds one sentence of reason to 00 0.5.
- SC13 adds two smoke steps to the build order.
- SC17 keeps an existing deferral and drops two phrases.
- AR1 to AR3, AR8, AR9, AR11, AR15, AR17, AR18 and AR21 are accepted
  risks or measurements applied as text of 05 5.4 and the pages.
- AR5 is a hook with its limit stated.
- AR22 is one reversible try, recorded in 05 5.4.
- AR23 leaves its decision until the probe A14 reports.
- AR4, AR6, AR12 and AR16 are held by ADR 0009 and AR7, AR19 and AR20
  fell with the ledger (0023).

The task also edits the pages its acceptance reaches and `owns` did not
name: the table of 12 12.5 in `docs/spec/12-engineering.md`, the
sentence of 00 0.4 that points at that table, and the task files of
T035, T040 and T077 that cite the records. The generated index
`docs/adr/README.md` is owned by no task (rule 1 of `docs/plan.md`).

Re-plan (2026-10-17): the row "stdlib `flag`, no CLI framework" left
the table of 12 12.5 for ADR 0015, which replaces that decision with
cobra, and took the number 0015; the six records of this task take the
numbers `tools/new adr` gives from 0016, under the collision rule of 12
12.3.

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
- Verify: `go run ./tools/ci generated` exits 0.
