---
id: T028
title: "The records of the questions block A settles"
milestone: M6
module: probes
fate: deferred
owns: []
needs:
  - T026
after: []
ask_first:
  - decisions
sitting: none
---

# T028 - The records of the questions block A settles

Re-plan (2026-10-12): deferred to M6; the records follow the results.
Reopened by: a real-sbx behaviour the fake cannot model breaks a daily
run; or the first release candidate. The text below is the one written
before the re-plan; `needs` lists its earlier dependencies with a folded
id replaced by the id that absorbed it, and `owns` is written when the
task reopens.

- Implements: REQ-0979 (11 (opening)), REQ-1030 (11 11.4), REQ-1168 (12
  12.5), REQ-1169 (12 12.5), REQ-1170 (12 12.5), REQ-1171 (12 12.5).
- Operator: no.
- Ask-first, as forecast before the re-plan: `decisions`.
- Acceptance:
  - Each question that block A settles alone, by the "Settled by" column
    of the index, leaves the table: Q7 (A11), Q15 (A4), Q16 (A13), Q18
    (A10), Q20 (A12), Q21 (A4), Q28 (A12) and Q32 (A14); it becomes an
    ADR when the decision is expensive to reverse or its probe
    overturned the default (index, Open questions; 12 12.5).
  - A question whose probe is arch-sensitive is settled only with a
    result from each host (11, opening).
  - The review may ask for one pull request per record; the task is all
    of them.
- Verify: `go run ./tools/ci generated`.
