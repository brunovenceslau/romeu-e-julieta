---
id: T108
title: "The re-plan record and the spec amendments"
milestone: M0
module: docs
fate: new
owns:
  - docs/adr/0012-*.md
  - docs/spec/02-layouts.md
  - docs/spec/10-testing-style.md
  - docs/spec/11-host-probes.md
  - docs/spec/12-engineering.md
needs: []
after:
  - T104
  - T105
  - T107
ask_first:
  - decisions
sitting: none
---

# T108 - The re-plan record and the spec amendments

Goal: One decision record, "re-plan v1 for early daily use: task files,
registration by file, build order, dogfood before recording", and the
one-line edits of 02 2.1, 10 10.1, 10 10.2, 10 10.3, 11 (its opening
and 11.1), 12 12.2 and 12 12.3 that "Spec sentences the re-plan amends"
in `docs/plan.md` lists, the `plan.md` tree row of 02 2.1 (amendment
1), the `ci-bootstrap` row and bootstrap step 3 (amendment 12), the
`internal/preflight/` and `catalog/` rows of 02 2.1 (amendment 13), the
container row of 10 10.1 (amendment 14) and the row of 12 12.3 that
lists the generated ask-first page (amendment 8, after T107) among
them. The pages it edits are on no ask-first surface; the record
is on `decisions`, and `docs/adr/README.md` is regenerated.

- Implements: 12 12.2 (Re-planning); ADR 0001; ADR 0005.
- Operator: no
- Acceptance:
  - Every anchor the plan and the decision records cite still resolves
    (a one-off script, run outside the tree).
  - The plan and the specification agree on every sentence that list
    names.
  - The record cites, for each practice it adopts, the earlier decision
    it supersedes and the trigger that reopens it.
- Verify: `go run ./tools/ci generated` exits 0 after `go generate
  ./...`; `go run ./tools/ci fast` exits 0.
