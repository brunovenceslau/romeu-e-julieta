---
id: T132
title: "Step durations and the `fast` budget"
milestone: M1
module: ci-release
fate: merged
owns:
  - tools/ci/internal/check/printer*.go
  - tools/ci/internal/check/env*.go
  - tools/ci/internal/check/step*.go
  - tools/ci/fast*.go
  - tools/ci/all*.go
  - tools/ci/main_test.go
  - tools/ci/internal/vulnerabilities/**
  - tools/ci/internal/setup/setup_test.go
  - tools/ci/internal/license/license_test.go
needs: []
after:
  - T105

ask_first:
  - checks
sitting: none
merged: 2026-10-18
pr: 150
---

# T132 - Step durations and the `fast` budget

Merged on 2026-10-18 in pull request #150. The text below is the one
written at the re-plan.

Goal: Each step prints one verdict line that ends with its duration, `ok
<name> (<d>)` with `<d>` rounded to 100 ms; `fast` warns on stderr when
it runs over three minutes ("fast: took <d>, over the three-minute
target of 10 10.2"), tested with an injected clock, never by sleeping;
the `vulnerabilities` step prints, when the network is unreachable, that
this `all` is not the equivalent run; a step that fails names itself, so
no run ends with a bare `exit status 1` (the output defect of pending
item 2b55a042).

Re-plan (2026-10-18): Changed. No `run` package exists since T105's
split: the check package prints the verdict lines (`check.Printer`), so
T132 owns `tools/ci/internal/check/printer*.go` in place of
`tools/ci/internal/run/progress*.go`, with `env*.go`, where the clock
is injected next to the other seams of `check.Env`, and `step*.go`, in
case a step carries the network signal. It also owns the files the four
behaviours reach: `fast` and `all` with their tests (the budget warning,
and the last line that names each failed step), `main_test.go`, whose
`runCI` gives those tests a fixed clock, the vulnerabilities
step (the line that says a run without the network is not the
equivalent run), and the setup and license tests, which pin a verdict
line. T125 comes after T132, since T125 owns
`tools/ci/internal/license/**`, which holds the license test.

Re-plan (2026-10-12, stage B): New. The progress line of 10 10.2 had no
task on `main`. `after: T105`, for the moved file layout, is an order
and not a data dependency (rule 2).

- Implements: REQ-0831 (10 10.2), REQ-0864 (10 10.2).
- Operator: no
- Acceptance:
  - The four behaviours, one test each, the clock injected (10 10.4).
- Verify: `go test ./tools/ci/... -run '^TestProgress'`.

Re-plan (2026-10-21): changed by T157. `fast` runs the race run (REQ-0833) in place of `unit`, and `vulnerabilities` (REQ-0831) runs before `race`; the budget warning of `fast` is unchanged, and the race run joined `fast` on a warm measurement of 118 s against the three minutes of `fastBudget`.
