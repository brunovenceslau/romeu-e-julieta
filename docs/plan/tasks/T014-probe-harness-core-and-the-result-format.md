---
id: T014
title: "Probe harness core and the result format"
milestone: M6
module: probes
fate: deferred
owns: []
needs:
  - T105
  - T125
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
names the tasks it reads code from today (the registry of T105, the
schema generator of T125), and `owns` is written when the task reopens.

- Implements: REQ-0733 (10 10.1), REQ-0734 (10 10.1), REQ-0750 (10
  10.1), REQ-0764 (10 10.1), REQ-0935 (10 10.3), REQ-0976 (11
  (opening)), REQ-0978 (11 (opening)), REQ-1021 (11 11.3), REQ-1022 (11
  11.3), REQ-1023 (11 11.3), REQ-1024 (11 11.3), REQ-1025 (11 11.3),
  REQ-1041 (12 12.1), REQ-1044 (12 12.1), REQ-1079 (12 12.2), REQ-1085
  (12 12.2), REQ-1119 (12 12.3), REQ-1122 (12 12.3).
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
  - `host.upstream` holds, for each probe the 11.4 table lists under a
    pin, that pin's version or digest, and `agent` for C3 and C5; empty
    otherwise.
  - A block A result at an sbx version above the floor is written to
    `docs/probes/<id>-<host>-<sbxVersion>.json` beside the floor's.
  - A `check` records each value its predicate evaluated as an
    observation, through the redaction, and a test recomputes the
    verdict from the file.
- Verify: `go test ./e2e/probes/...`; `go run ./tools/ci schema`.
