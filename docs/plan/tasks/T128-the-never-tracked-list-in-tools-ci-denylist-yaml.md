---
id: T128
title: "The never-tracked list in `tools/ci/denylist.yaml`"
milestone: M1
module: ci-release
fate: new
owns:
  - tools/ci/denylist.yaml
  - tools/ci/internal/hygiene/nevertracked*.go
  - tools/ci/names/names*.go
  - tools/ci/internal/hygiene/hygiene*.go
  - tools/ci/internal/pushedrange/pushedrange*.go
  - tools/ci/internal/citest/citest.go
  - tools/ci/fast_test.go
  - tools/ci/internal/hygiene/add_test.go
needs: []
after:
  - T105

ask_first:
  - checks
  - approvals
sitting: none
---

# T128 - The never-tracked list in `tools/ci/denylist.yaml`

Goal: The never-tracked file names read from `tools/ci/denylist.yaml`
instead of the Go constant (`prologueName`, `tools/ci/hygiene.go` on
`main`), the change round 8 left as R8-10-34.

Re-plan (2026-10-18): Changed. `owns` adds the three places the list
reaches: `tools/ci/names/names*.go`, which decodes `denylist.yaml`
strictly and rewrites it through `hygiene add`, so the new key is
declared, kept and checked there; and `hygiene*.go` and
`pushedrange*.go`, the two rules that call `IsPrologue` today; and
`tools/ci/internal/citest/citest.go`, whose fixture denylist gains the
list, and `tools/ci/fast_test.go` and `tools/ci/internal/hygiene/add_test.go`,
whose cases read it.

Re-plan (2026-10-12, stage B): New. A change to merged work is a task,
so it has an owner and an `after` edge: `after: T105`, for the moved
file layout, is an order and not a data dependency (rule 2).

`tools/ci/denylist.yaml` is on the `approvals` surface as well as on
`checks` (05 5.3), so both lines are forecast.

- Implements: REQ-0825 (10 10.2), REQ-0883 (10 10.2).
- Operator: no
- Acceptance:
  - A fixture name in the list fails `hygiene` at any depth and case.
  - A pushed commit that adds then removes it is refused.
- Verify: `go test ./tools/ci/... -run '^TestNeverTracked'`.
