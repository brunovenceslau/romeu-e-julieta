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

- Implements: REQ-0110 (01 1.7), REQ-0263 (03 3.13), REQ-0264 (03 3.13),
  REQ-0265 (03 3.13), REQ-0266 (03 3.13), REQ-0830 (10 10.2), REQ-1112
  (12 12.3).
- Operator: no.
- Ask-first, as forecast before the re-plan: `checks`.
- Acceptance:
  - The denylist is generated from the table of 01 1.7: `go generate`
    writes the Not column to `docs/reference/vocabulary.yaml`, which no
    `checks` glob of `.github/ask-first.yaml` matches (a test reads the
    globs).
  - A "Not" phrase in a doc, in help text or in an error message fails;
    `docs/reviews/`, `docs/adr/` and the "Not" column itself are exempt.
- Verify: `go test ./tools/ci/... -run '^TestVocabulary'`, which runs
  the fixture lines; `go run ./tools/ci vocabulary`.
