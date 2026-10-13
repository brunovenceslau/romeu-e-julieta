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
T009. The rows it reads name `procgroup` in the third row and the
sentence of 10 10.7 that `tools/**` and `e2e/**` reach the command
registries through `internal/cmd/<binary>/all` only.

- Implements: REQ-0001 (index), REQ-0017 (index), REQ-0026 (index),
  REQ-0039 (01 1.1), REQ-0114 (02 2.1), REQ-0482 (05 5.2), REQ-0483 (05
  5.2), REQ-0508 (05 5.2), REQ-0839 (10 10.2), REQ-0959 (10 10.6),
  REQ-0968 (10 10.7), REQ-0969 (10 10.7), REQ-0970 (10 10.7), REQ-0971
  (10 10.7), REQ-0972 (10 10.7), REQ-1233 (10 10.7).
- Operator: no.
- Acceptance:
  - The check enforces the table of 10 10.7 over the packages that
    exist, and a fixture package fails for each column.
  - A package under `internal/` that no row names fails, with a fixture.
  - `tools/ci` importing a command package other than through
    `internal/cmd/<binary>/all` fails, with a fixture.
  - It carries the I1 and I2 guard tags, each with a one-violation
    fixture as its tagged test (05 5.2): `go list -deps ./cmd/romeu`
    holds none of `internal/layout`, `internal/hooks`, `internal/mise`.
- Verify: `go test ./tools/ci/... -run '^TestImports'`, which runs the
  fixture packages; `go run ./tools/ci imports`.
