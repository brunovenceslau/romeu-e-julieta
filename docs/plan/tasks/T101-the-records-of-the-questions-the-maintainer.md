---
id: T101
title: "The records of the questions the maintainer settles"
milestone: M6
module: docs
fate: deferred
owns: []
needs:
  - T028
after: []
ask_first:
  - decisions
sitting: none
---

# T101 - The records of the questions the maintainer settles

Re-plan (2026-10-12): deferred to M6; the review files hold the
decisions. Reopened by: the first release candidate; or one record when
its decision is first touched by code. The text below is the one written
before the re-plan; `needs` names T028 only, no chain through the other
record tasks, and `owns` is written when the task reopens.

- Implements: REQ-0015 (index), REQ-1168 (12 12.5), REQ-1169 (12 12.5),
  REQ-1170 (12 12.5), REQ-1171 (12 12.5).
- Operator: the maintainer's decision on each.
- Ask-first, as forecast before the re-plan: `decisions`.
- Acceptance:
  - Each question whose "Settled by" cell is the maintainer leaves the
    table: Q4, Q6, Q9, Q12, Q13, Q14, Q26, Q27, Q29, Q30 and Q31; it
    becomes an ADR when the decision is expensive to reverse or its
    probe overturned the default (index, Open questions; 12 12.5). Q25
    left it when it was decided on 2026-10-07, recorded in [ADR 0008,
    let the sandbox act as the maintainer on
    GitHub](../../adr/0008-let-the-sandbox-act-as-the-maintainer-on-github.md)
    from the output of block O1, and Q23 became the registry-credential
    row of Deferred decisions.
  - After this task the Open questions table is empty, or each remaining
    row says what it waits for.
- Verify: `go run ./tools/ci generated`; `go run ./tools/ci sequences`
  once T004 lands.
