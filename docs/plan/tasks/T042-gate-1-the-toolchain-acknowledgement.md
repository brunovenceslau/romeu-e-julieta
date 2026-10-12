---
id: T042
title: "Gate 1: the toolchain acknowledgement"
milestone: M3
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/approve/**
  - cmd/romeu/import_approve.go
  - e2e/approve_test.go
needs:
  - T029
  - T033
  - T031
  - T103
after: []
ask_first: []
sitting: none
---

# T042 - Gate 1: the toolchain acknowledgement

Goal: Gate 1, `approve --toolchain`.

Re-plan (2026-10-12): Kept whole: a security invariant is never deferred
(ADR 0005, rule 1).

- Implements: 01 1.4 (gate 1), 04 4.2 (`approve --toolchain`), 03 3.8,
  09 (J9), 05 5.2 (I7).
- Operator: no.
- Acceptance:
  - `approve --toolchain` needs a terminal, prints the julieta version
    change and the catalog diff per affected project before the prompt,
    and writes `toolchain.json`.
  - A unit table shows that gate 1 flips on a different `sbx` version, a
    different romeu version and a different catalog digest (I7).
  - The check function is the one the preflight and `sync` call;
    `status` and `doctor` report a mismatch instead of exiting 3.
- Verify: `go test -tags e2e ./e2e/... -run '^TestApprove' -v`. The
  pattern names the `TestApprove*` functions this task adds, and the
  reviewer reads them in the `-v` output (10 10.8).
