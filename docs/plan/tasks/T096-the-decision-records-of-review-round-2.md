---
id: T096
title: "The decision records of review round 2"
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

# T096 - The decision records of review round 2

Re-plan (2026-10-12): deferred to M6; the review files hold the
decisions. Reopened by: the first release candidate; or one record when
its decision is first touched by code. The text below is the one written
before the re-plan; `needs` names T028 only, no chain through the other
record tasks, and `owns` is written when the task reopens.

- Implements: REQ-0015 (index), REQ-1169 (12 12.5), REQ-1170 (12 12.5),
  REQ-1171 (12 12.5).
- Operator: no.
- Ask-first, as forecast before the re-plan: `decisions`.
- Acceptance:
  - A record, written with `go run ./tools/new adr`, exists for each
    decision under "Settled questions" of `docs/reviews/round-2.md` that
    the records under `docs/adr/` do not already hold (12 12.5).
  - `tools/ci generated` passes, and `tools/ci sequences` once T004
    lands. That the records cover the round's decisions is **[review]**
    (S10): the pull request's Evidence lists each heading under that
    section with the record that holds it, so the count of records
    against headings is read there and not inferred.
- Verify: `go run ./tools/ci generated`; `go run ./tools/ci sequences`
  once T004 lands.
