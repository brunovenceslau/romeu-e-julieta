---
id: T100
title: "The decision records of review round 6"
milestone: M6
module: docs
fate: deferred
owns: []
needs:
  - T099
after: []
ask_first:
  - decisions
sitting: none
---

# T100 - The decision records of review round 6

Re-plan (2026-10-12): deferred to M6; the review files hold the
decisions. Reopened by: the first release candidate; or one record when
its decision is first touched by code. The text below is the one written
before the re-plan; `needs` lists its earlier dependencies with a folded
id replaced by the id that absorbed it, and `owns` is written when the
task reopens.

- Implements: 12 12.5, S10, ADR 0001 rules 6 to 9.
- Operator: no.
- Ask-first, as forecast before the re-plan: `decisions`.
- Acceptance:
  - A record, written with `go run ./tools/new adr`, exists for each
    decision under "Maintainer decisions" of `docs/reviews/round-6.md`
    that the six accepted records do not already hold (12 12.5).
  - `tools/ci sequences` and `tools/ci generated` pass. That the records
    cover the round's decisions is **[review]** (S10): the pull
    request's Evidence lists each heading under that section with the
    record that holds it, so the count of records against headings is
    read there and not inferred.
- Verify: `go run ./tools/ci sequences`; `go run ./tools/ci generated`.
