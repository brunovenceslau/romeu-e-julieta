---
id: T129
title: "Fixed module sources for `tools/ci setup`"
milestone: M1
module: ci-release
fate: merged
owns:
  - tools/ci/internal/setup/**
needs: []
after:
  - T105
  - T141

ask_first:
  - checks
sitting: none
merged: 2026-10-17
pr: 102
---

# T129 - Fixed module sources for `tools/ci setup`

Merged on 2026-10-17 in pull request #102. The text below is the one
written at the re-plan.

Goal: `setup`'s three steps, `mise trust`, `mise install` and `go mod
download`, run with `GOPROXY=https://proxy.golang.org`,
`GOSUMDB=sum.golang.org` and `GOENV=off` (REQ-0862, 10 10.2), the
change round 8 left as R8-10-38, and the checksum database guards both
installs of govulncheck (12 12.1).

Re-plan (2026-10-12, stage B): New. A change to merged work is a task,
so it has an owner and an `after` edge: `after: T105`, for the moved
file layout, is an order and not a data dependency (rule 2).

- Implements: REQ-0862 (10 10.2), REQ-1071 (12 12.1).
- Operator: no
- Acceptance:
  - A test shows a caller's `GONOSUMDB` and `GOFLAGS` do not reach any
    of the three steps.
- Verify: `go test ./tools/ci/... -run '^TestSetupEnv'`.
