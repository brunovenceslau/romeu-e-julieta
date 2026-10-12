---
id: T073
title: "The meta-tests"
milestone: M4
module: romeu-cli
fate: kept
owns:
  - e2e/meta_*_test.go
  - e2e/internal/meta/**
  - internal/cli/errorids_exempt.yaml
needs:
  - T069
  - T067
  - T070
  - T049
after: []
ask_first: []
sitting: none
---

# T073 - The meta-tests

Goal: The meta-tests: idempotency rows read from the definitions,
concurrency, promotion and destructive fault injection, error ids
asserted, no network in the preflight.

Re-plan (2026-10-12): Kept as written before the re-plan.

- Implements: 10 10.1 ("Additional required tests"), 04 4.1
  (Idempotency, Locking), 01 1.6, 05 5.2 (I7).
- Operator: no.
- Acceptance:
  - The idempotency table T043 started has a row for each converging and
    each appending command of 04 4.1, romeu's and julieta's (`setup`,
    `install`, `lock`, `memory add`, `handoff write` and `snapshot`, at
    the container level); a test fails a command of the
    command definitions that has no row.
  - Two romeu processes: the second exits 1 with `RJ-101`.
  - Promotion fault injection: after a kill at each step, each **P**
    command finishes the promotion or exits 4, and a re-sync converges
    (I7).
  - Each **P** command runs with the registry and the origins
    unreachable after `sync`, and succeeds.
- Verify: `go test -tags e2e ./e2e/... -run '^TestMeta' -v`. The pattern
  names the `TestMeta*` functions this task adds, and the reviewer reads
  them in the `-v` output (10 10.8).
