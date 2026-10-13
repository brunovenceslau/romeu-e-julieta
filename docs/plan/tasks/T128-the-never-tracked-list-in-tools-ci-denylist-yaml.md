---
id: T128
title: "The never-tracked list in `tools/ci/denylist.yaml`"
milestone: M1
module: ci-release
fate: new
owns:
  - tools/ci/denylist.yaml
  - tools/ci/internal/hygiene/nevertracked*.go
needs:
  - T105
after: []
ask_first:
  - checks
sitting: none
---

# T128 - The never-tracked list in `tools/ci/denylist.yaml`

Goal: The never-tracked file names read from `tools/ci/denylist.yaml`
instead of the Go constant (`prologueName`, `tools/ci/hygiene.go` on
`main`), the change round 8 left as R8-10-34.

Re-plan (2026-10-12, stage B): New. A change to merged work is a task,
so it has an owner and an `after` edge.

- Implements: REQ-0825 (10 10.2), REQ-0883 (10 10.2).
- Operator: no
- Acceptance:
  - A fixture name in the list fails `hygiene` at any depth and case.
  - A pushed commit that adds then removes it is refused.
- Verify: `go test ./tools/ci/... -run '^TestNeverTracked'`.
