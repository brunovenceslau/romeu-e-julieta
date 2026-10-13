---
id: T093
title: "The records of the questions block B settles"
milestone: M6
module: probes
fate: deferred
owns: []
needs:
  - T092
after: []
ask_first:
  - decisions
sitting: none
---

# T093 - The records of the questions block B settles

Re-plan (2026-10-12): deferred to M6; release only. Reopened by: a
real-sbx behaviour the fake cannot model breaks a daily run; or the
first release candidate. The text below is the one written before the
re-plan; `needs` lists its earlier dependencies with a folded id
replaced by the id that absorbed it, and `owns` is written when the task
reopens.

- Implements: REQ-1030 (11 11.4), REQ-1088 (12 12.2), REQ-1168 (12
  12.5), REQ-1169 (12 12.5), REQ-1170 (12 12.5), REQ-1171 (12 12.5).
- Operator: no.
- Ask-first, as forecast before the re-plan: `decisions`.
- Acceptance:
  - Each of the four questions whose last probe is in block B leaves the
    table: Q2 (A12, B2), Q8 (C3, B2), Q19 (A12, B2) and Q22 (C5, B3); it
    becomes an ADR when the decision is expensive to reverse or its
    probe overturned the default (index, Open questions; 12 12.5).
  - The J5 guide page gains the sentence on the hook order C5 recorded
    (Q22), a post-block-B edit of that page.
  - The resolution of an overturned result is part of T092, not of this
    task.
- Verify: `go run ./tools/ci generated`; `go run ./tools/ci probes`.
