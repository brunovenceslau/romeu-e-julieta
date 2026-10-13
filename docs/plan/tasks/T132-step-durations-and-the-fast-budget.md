---
id: T132
title: "Step durations and the `fast` budget"
milestone: M1
module: ci-release
fate: new
owns:
  - tools/ci/internal/run/progress*.go
needs:
  - T105
after: []
ask_first:
  - checks
sitting: none
---

# T132 - Step durations and the `fast` budget

Goal: Each step prints one verdict line that ends with its duration, `ok
<name> (<d>)` with `<d>` rounded to 100 ms; `fast` warns on stderr when
it runs over three minutes ("fast: took <d>, over the three-minute
target of 10 10.2"), tested with an injected clock, never by sleeping;
the `vulnerabilities` step prints, when the network is unreachable, that
this `all` is not the equivalent run; a step that fails names itself, so
no run ends with a bare `exit status 1` (the output defect of pending
item 2b55a042).

Re-plan (2026-10-12, stage B): New. The progress line of 10 10.2 had no
task on `main`.

- Implements: REQ-0831 (10 10.2), REQ-0864 (10 10.2).
- Operator: no
- Acceptance:
  - The four behaviours, one test each, the clock injected (10 10.4).
- Verify: `go test ./tools/ci/... -run '^TestProgress'`.
