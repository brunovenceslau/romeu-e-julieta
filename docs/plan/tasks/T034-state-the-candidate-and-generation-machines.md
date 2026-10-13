---
id: T034
title: "`state`: the candidate and generation machines"
milestone: none
module: state
fate: folded
absorbed_by: T033
owns: []
needs: []
after: []
ask_first: []
sitting: none
---

# T034 - `state`: the candidate and generation machines

Re-plan (2026-10-12): folded into T033; its rows are copied into T033's
Acceptance. This file keeps the id so that earlier citations resolve;
the text below is the one written before the re-plan, and T033 is where
it is built.

- Implements: 01 1.6, 12 12.3 (the transition-table row), 05 5.2 (I7 and
  I17, unit level).
- Operator: no.
- Ask-first, as forecast before the re-plan: `gates`.
- Acceptance:
  - Each machine is a transition table; the generated tests exercise
    each row and assert each missing pair illegal, with exit 2 and an
    error id.
  - The candidate and generation diagrams that `ARCHITECTURE.md` links
    are generated from the tables, each in its own file under
    `docs/diagrams/` (12 12.3). They are the only generated diagrams:
    the promotion commit is the numbered procedure of 01 1.6, and the
    probe has no lifecycle diagram (12 12.8).
- Verify: `go test ./internal/state/...`; `go run ./tools/ci generated`.
