---
id: T066
title: "`romeu run` and the version handshake"
milestone: M3
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/run/**
  - cmd/romeu/import_run.go
  - e2e/scenarios/j3/**
  - e2e/scenarios/j2/step4_*.go
  - e2e/hybrid_test.go
  - e2e/run_test.go
  - internal/cmd/romeu/status/julieta_fields.go
needs:
  - T043
  - T046
  - T051
  - T054
  - T079
  - T103
  - T013
after:
  - T048
ask_first: []
sitting: none
---

# T066 - `romeu run` and the version handshake

Earlier title: `romeu run`.

Goal: `romeu run`: the generation record, egress reconcile, the
compatibility check of 06 6.3, `julieta setup`, `sbx env run -d`, the
lock released before `exec`, J3, and the hybrid test.

Re-plan (2026-10-12): Kept, changed; T016 and T030 (the version
handshake) fold in as its hybrid test, `e2e/hybrid_test.go`, with their
acceptance rows as written there. `run` attaches a single pane until
`layout` (T056) lands: 04 4.2 step 7 is reached by no command, dark.

- Implements: 04 4.2 ("How `romeu run` reaches the run layout"), 01 1.5,
  01 1.6 ("Generation"), 07 7.5, 09 (J3, J10), S8, 05 5.2 (I1, I7, I30).
- Operator: no.
- Acceptance:
  - Hybrid, on the Linux runners, the J3b scenario function passes: a
    generation is recorded before `sbx env run`, egress is reconciled,
    the compatibility check and `setup` run, and the final argv equals
    the one of I1 byte for byte.
  - The 32 KiB manifest round-trip of I30 runs against the recorded
    `--env` shape of T030 and the manifest reader of T051, which this
    task is the first to hold together.
  - The J3a variant: with a sandbox that exists, `run` starts or
    reattaches with `sbx env run -d` without re-provisioning, and
    `julieta setup` is a no-op when the locks are unchanged (09, J3;
    S8).
  - A failed create removes the record when the sandbox is absent and
    keeps it when present. An absent sandbox with an open generation is
    preserved first (04 4.2, `run` step 2); what that preservation
    imports is asserted in T068, with `result: lost`.
  - A memory directory that violates the layout allowlist makes `run`
    exit 2 (I24).
  - A failed compatibility check stops `run` with exit 2 (I30).
  - A gate 1 or a gate 2 change makes `run` exit 3 before a mutating
    call (I7).
  - romeu's share of S8 is measured by B5 on real hosts (11 11.2), and
    no CI test asserts a bound on it (S8).
  - The lock is released before the final `exec` (04 4.1).
  - `romeu status` gains julieta's warnings and recorded hook failures
    for a running sandbox, and says so when it skips that exec (04 4.2),
    in this task's own file under `internal/cmd/romeu/status/`, after
    T048, asserted in `e2e/run_test.go` against the sandbox `run`
    started.
  - The J2 scenario function gains its step 4, `romeu run`, in this
    task's own step file under `e2e/scenarios/j2/`.
- Verify: `go test -tags e2e ./e2e/... -run '^(TestRun|TestHybrid)' -v`.
  The pattern names the `TestRun*` and `TestHybrid*` functions this task
  adds, and the reviewer reads them in the `-v` output (10 10.8).
