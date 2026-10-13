---
id: T129
title: "Fixed module sources for `tools/ci setup`"
milestone: M1
module: ci-release
fate: new
owns:
  - tools/ci/internal/setup/**
needs: []
after:
  - T105

ask_first:
  - checks
sitting: none
---

# T129 - Fixed module sources for `tools/ci setup`

Goal: `setup`'s `go mod download` runs with
`GOPROXY=https://proxy.golang.org`, `GOSUMDB=sum.golang.org` and
`GOENV=off`, the change round 8 left as R8-10-38, and the checksum
database guards both installs of govulncheck (12 12.1).

Re-plan (2026-10-12, stage B): New. A change to merged work is a task,
so it has an owner and an `after` edge: `after: T105`, for the moved
file layout, is an order and not a data dependency (rule 2).

- Implements: REQ-1071 (12 12.1).
- Operator: no
- Acceptance:
  - A test shows a caller's `GONOSUMDB` and `GOFLAGS` do not reach the
    download.
- Verify: `go test ./tools/ci/... -run '^TestSetupEnv'`.
