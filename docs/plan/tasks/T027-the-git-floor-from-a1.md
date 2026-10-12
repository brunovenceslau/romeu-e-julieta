---
id: T027
title: "The git floor from A1"
milestone: M6
module: gitsafe
fate: deferred
owns: []
needs:
  - T026
  - T023
after: []
ask_first:
  - gitsafe
  - decisions
sitting: none
---

# T027 - The git floor from A1

Re-plan (2026-10-12): deferred to M6; the git floor is read on the host
at sitting M2 and written into T023's floor file as a fixture with
`source: sitting M2`. Reopened by: a real-sbx behaviour the fake cannot
model breaks a daily run; or the first release candidate. The text below
is the one written before the re-plan; `needs` lists its earlier
dependencies with a folded id replaced by the id that absorbed it, and
`owns` is written when the task reopens.

- Implements: 05 5.1 (the host git floor), index (Open questions, Q10),
  12 12.5.
- Operator: no.
- Ask-first, as forecast before the re-plan: `gitsafe`, `decisions`.
- Acceptance:
  - `internal/gitsafe/floor.go` holds the per-series minimums A1
    verified, with sources.
  - Q10 leaves the Open questions table and becomes an ADR.
- Verify: `go test ./internal/gitsafe/...`; `go run ./tools/ci
  sequences`.
