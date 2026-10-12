---
id: T011
title: "`tools/ci vocabulary`"
milestone: M6
module: ci-release
fate: deferred
owns: []
needs:
  - T009
after: []
ask_first:
  - checks
sitting: none
---

# T011 - `tools/ci vocabulary`

Re-plan (2026-10-12): deferred to M6; polish. Reopened by: the first
release candidate. The text below is the one written before the re-plan;
`needs` lists its earlier dependencies with a folded id replaced by the
id that absorbed it, and `owns` is written when the task reopens.

- Implements: 01 1.7, 10 10.2 (the `vocabulary` step), 12 12.3 (the
  vocabulary row).
- Operator: no.
- Ask-first, as forecast before the re-plan: `checks`.
- Acceptance:
  - The denylist is generated from the table of 01 1.7.
  - A "Not" phrase in a doc, in help text or in an error message fails;
    `docs/reviews/` and the "Not" column itself are exempt.
- Verify: `go test ./tools/ci/... -run '^TestVocabulary'`, which runs
  the fixture lines; `go run ./tools/ci vocabulary`.
