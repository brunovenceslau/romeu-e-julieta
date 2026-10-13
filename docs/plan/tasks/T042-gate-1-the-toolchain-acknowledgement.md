---
id: T042
title: "Gate 1: the toolchain acknowledgement"
milestone: M3
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/approve/**
  - internal/cmd/romeu/all/import_approve.go
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

- Implements: REQ-0055 (01 1.3), REQ-0059 (01 1.4), REQ-0060 (01 1.4),
  REQ-0061 (01 1.4), REQ-0062 (01 1.4), REQ-0063 (01 1.4), REQ-0065 (01
  1.4), REQ-0228 (03 3.8), REQ-0272 (04 4.1), REQ-0310 (04 4.2),
  REQ-0369 (04 4.2), REQ-0488 (05 5.2), REQ-0638 (07 7.6), REQ-0753 (10
  10.1), REQ-0754 (10 10.1), REQ-0756 (10 10.1), REQ-0757 (10 10.1),
  REQ-0758 (10 10.1), REQ-0759 (10 10.1), REQ-0790 (10 10.1).
- Operator: no.
- Acceptance:
  - `approve --toolchain` needs a terminal, prints the julieta version
    change and the catalog diff per affected project before the prompt,
    and writes `toolchain.json`; the prompt goes to stderr, and stdin
    decides the TTY.
  - A unit table shows that gate 1 flips on a different `sbx` version,
    romeu version, romeu sha256, embedded julieta sha256, a different
    julieta version or protocol, and catalog digest, and that the prompt
    prints the live sbx version beside the newest recorded one and
    `downgrade` beside a version lower than the recorded one (I7, 01
    1.4); a build reports `v0.0.0-dev+<commit>`, is acknowledged once
    per build per machine, and with julieta and the catalog unchanged
    the prompt is one line.
  - Gate 1's print names the tested window of 10 10.3.
  - The check function is the one the preflight and `sync` call;
    `status` and `doctor` report a mismatch instead of exiting 3.
- Verify: `go test -tags e2e ./e2e/... -run '^TestApprove' -v`. The
  pattern names the `TestApprove*` functions this task adds, and the
  reviewer reads them in the `-v` output (10 10.8).
