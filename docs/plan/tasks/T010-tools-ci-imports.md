---
id: T010
title: "`tools/ci imports`"
milestone: M1
module: ci-release
fate: kept
owns:
  - tools/ci/internal/imports/**
  - tools/ci/check_imports.go
needs:
  - T009
  - T105
after: []
ask_first:
  - checks
sitting: none
---

# T010 - `tools/ci imports`

Goal: `tools/ci imports`: the table of 10 10.7 over the packages that
exist, and the I1, I2 and I24 guards.

Re-plan (2026-10-12): Kept, pulled forward: its first input exists at
T009.

- Implements: 10 10.7, 10 10.2 (the `imports` step), 05 5.2 (I1, I2).
- Operator: no.
- Acceptance:
  - The check enforces the table of 10 10.7 over the packages that
    exist, and a fixture package fails for each column.
  - It carries the I1 and I2 guard tags, each with a one-violation
    fixture as its tagged test (05 5.2): `go list -deps ./cmd/romeu`
    holds none of `internal/layout`, `internal/hooks`, `internal/mise`.
- Verify: `go test ./tools/ci/... -run '^TestImports'`, which runs the
  fixture packages; `go run ./tools/ci imports`.
