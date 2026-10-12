---
id: T053
title: "The mise driver: `install`, `lock` and `tools/ci mise`"
milestone: M2
module: julieta-core
fate: kept
owns:
  - internal/mise/**
  - internal/cmd/julieta/install/**
  - internal/cmd/julieta/lock/**
  - internal/cmd/julieta/setup/install.go
  - cmd/julieta/import_install.go
  - cmd/julieta/import_lock.go
  - e2e/julieta_install_test.go
  - e2e/julieta_lock_test.go
  - internal/hooks/actions/lock_check.go
  - tools/ci/internal/mise/**
  - tools/ci/check_mise.go
  - go.mod
  - go.sum
needs:
  - T051
  - T012
  - T054
  - T105
after: []
ask_first:
  - checks
  - dependencies
sitting: none
---

# T053 - The mise driver: `install`, `lock` and `tools/ci mise`

Goal: The mise driver: `install`, `lock`, `lock --check`, the mise
environment of 04 4.3, `tools/ci mise`, and the `pre-commit` action
file.

Re-plan (2026-10-12): Kept. The TOML parser of 12 12.1 enters `go.mod`
here, with its first importer (rule 7 of `docs/plan.md`); `egress`
(T036) needs this task for it. The J8
scenario function of the last row below moves to T075 with the other
remaining scenario functions: the package it joins starts in T041
(M3), which this task (M2) does not wait for. The lock-change commit on
the fixture origin is asserted here by the container e2e instead.

- Implements: 04 4.3 (`install`, `lock`; the mise environment), 07 7.2,
  07 7.3, 10 10.2 (the `mise` step), 09 (J8), S8.
- Operator: no.
- Acceptance:
  - `install` runs with the environment of 04 4.3 and skips when the
    `lock-set` digest equals the last success; `--force` overrides; a
    `mise.toml` without a lock fails with the hint. `setup` gains
    `install` (04 4.3) in this task's own file under the `setup`
    package of T051.
  - `lock` passes the four platforms and rewrites `mise.lock`; `lock
    --check` writes nothing, fails a tool without a lock entry and a
    lockable entry without both linux platforms with exit 6
    (`check-findings`), and warns on a missing macOS entry.
  - `tools/ci mise` runs that logic on this repository's lock and the
    examples'.
  - The `julieta setup` no-op is at most 3 s, median of 5, in the
    container e2e against local origins (S8). The bound is S8's own, so
    a run over it is a defect to fix, never a flake to retry (10 10.1).
  - The J8 scenario function, added to the package T041 started, replays
    the agent's lock change as a commit on the fixture origin, which is
    its step 1. Its step 2, the egress outcome after `romeu sync`, needs
    T047 and a recording, so T075 asserts it; this task stays free of
    block O3.
- Verify: `go test -tags e2e ./e2e/... -run
  '^(TestJulietaInstall|TestJulietaLock)' -v`; `go run ./tools/ci mise`.
  The pattern names the `TestJulietaInstall*` and `TestJulietaLock*`
  functions this task adds, and the reviewer reads them in the `-v`
  output (10 10.8).
