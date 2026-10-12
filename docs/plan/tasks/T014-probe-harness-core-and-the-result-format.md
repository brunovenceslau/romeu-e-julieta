---
id: T014
title: "Probe harness core and the result format"
milestone: M6
module: probes
fate: deferred
owns: []
needs:
  - T006
after: []
ask_first:
  - checks
  - dependencies
sitting: none
---

# T014 - Probe harness core and the result format

Re-plan (2026-10-12): deferred to M6; the harness lands with the
recorder, after the redaction package has its unit tests; until then the
fake replays sessions written by hand (T029). Reopened by: a real-sbx
behaviour the fake cannot model breaks a daily run; or the first release
candidate. The text below is the one written before the re-plan; `needs`
lists its earlier dependencies with a folded id replaced by the id that
absorbed it, and `owns` is written when the task reopens.

- Implements: 11 (opening), 11 11.3, 12 12.3 (the `probe` kind), 10 10.1
  (Schema), 03 (opening).
- Operator: no.
- Ask-first, as forecast before the re-plan: `checks`, `dependencies`
  (the JSON Schema validator of 12 12.1).
- Acceptance:
  - The pure core computes `verdict` and `decision` from declared
    `onPass`, `onFail` or a decision table; a typed decision is refused
    (11 11.3).
  - `tools/schemagen` writes `schemas/probe-result.v1.json` from the Go
    type; `tools/ci schema` fails when the generated schema differs from
    the committed one and validates the examples.
  - `go run ./tools/new probe` writes a definition with `expect`, a
    decision table and `affects` to fill.
  - `<host>` in a result's file name (`docs/probes/<id>-<host>.json`) is
    `<os>-<arch>`, never the host's name; that reading is the plan's
    addition.
- Verify: `go test ./e2e/probes/...`; `go run ./tools/ci schema`.
