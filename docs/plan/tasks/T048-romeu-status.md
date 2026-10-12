---
id: T048
title: "`romeu status`"
milestone: M3
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/status/**
  - cmd/romeu/import_status.go
  - e2e/status_test.go
needs:
  - T043
  - T103
after: []
ask_first: []
sitting: none
---

# T048 - `romeu status`

Goal: `romeu status`.

Re-plan (2026-10-12): Kept, narrowed: drift, interrupted promotion,
stale toolchain, orphaned; julieta's fields join with T066.

- Implements: 04 4.2 (`status`), 09 (J7 step 2, J10 step 1, J13 step 3).
- Operator: no.
- Acceptance:
  - On fixtures of host state, `status --json` reports drift, an
    interrupted promotion, a stale toolchain, an
    orphaned project and an orphaned repo dir, without taking the lock.
  - The J10 step 1 line is a fixture: `absent; generation <G> open;
    snapshot <T>; refs/sandboxes last fetched <T2>`; for an orphaned
    repo dir the report lists its unpushed and salvage-only work (J13
    step 3).
  - It exits 0 or 1 and never 3.
  - The fields that come from julieta land with T066, in a file of its
    own under this package.
- Verify: `go test -tags e2e ./e2e/... -run '^TestStatus' -v`. The
  pattern names the `TestStatus*` functions this task adds, and the
  reviewer reads them in the `-v` output (10 10.8).
