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

Re-plan (2026-10-12): deferred to M6; until it lands, T023's floor file
holds the initial floor 2.45.4 of 05 5.1, and a host git version read at
sitting M2 that differs from it reaches the file by a one-line pull
request with `source: sitting M2` (T113). Reopened by: a real-sbx
behaviour the fake cannot model breaks a daily run; or the first release
candidate. The text below is the one written before the re-plan; `needs`
lists its earlier dependencies with a folded id replaced by the id that
absorbed it, and `owns` is written when the task reopens.

- Implements: REQ-0368 (04 4.2), REQ-0469 (05 5.1), REQ-1057 (12 12.1).
- Operator: no.
- Ask-first, as forecast before the re-plan: `gitsafe`, `decisions`.
- Acceptance:
  - `internal/gitsafe/floor.go` holds the per-series minimums A1
    verified, with sources.
  - Q10 leaves the Open questions table and becomes an ADR.
- Verify: `go test ./internal/gitsafe/...`; `go run ./tools/ci
  sequences`.
