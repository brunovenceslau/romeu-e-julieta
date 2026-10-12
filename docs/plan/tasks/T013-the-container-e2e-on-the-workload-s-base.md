---
id: T013
title: "The container e2e, on the workload's base"
milestone: M1
module: julieta-core
fate: kept
owns:
  - e2e/container_test.go
  - e2e/internal/container/**
  - tools/ci/internal/container/**
  - tools/ci/check_container.go
needs:
  - T009
  - T012
after: []
ask_first:
  - checks
sitting: none
---

# T013 - The container e2e, on the workload's base

Goal: The container e2e on the workload's base: the harness, the check
package, the image pulled by digest (Q27).

Re-plan (2026-10-12): Kept, in M1: it is the specification's first
julieta test level (10 10.1), and no new level is added. A transport
failure is an infrastructure error, a digest mismatch a test failure.
`e2e/container_test.go` is the harness entry; each julieta command's
container cases live in `e2e/julieta_<command>_test.go` and each kit's
in `e2e/kit_<name>_test.go`, owned by that task (rule 1, amendment 14),
so no two tasks append to one test file.

- Implements: 10 10.1 (E2E container), 12 12.1.
- Operator: no.
- Acceptance:
  - The container e2e runs `julieta version --json` in the workload's
    Debian base, pulled by the digest of the pin file, on
    `ubuntu-26.04` and `ubuntu-26.04-arm`, native.
  - A pull that fails on transport (DNS, a timeout, a 5xx) is reported
    as an infrastructure error, the way 10 10.1 treats a failed mise
    download: a red job with its own exit code, never a skip, and a
    fault test makes the pull fail that way and asserts the code. A
    manifest digest that differs from the pin is a test failure, always:
    it is the one signal that a pinned artifact was substituted, and a
    security gate is hardened, not retried.
  - `tools/ci all` gains the `e2e` step and the "build the tagged
    suites" step of 10 10.2 here, each registered by file (rule 4 of
    `docs/plan.md`), and the step-list golden of T105 changes in the
    same pull request; until T103 lands `tools/release build
    --dry-run`, the `e2e` step runs only the julieta cases, which need
    no romeu build; from here on every `-tags e2e` Verify line of a
    later task runs in CI.
  - `tools/ci all` selects the step from the OS and architecture it runs
    on (10 10.2).
- Verify: `go test -tags e2e ./e2e/... -run '^TestContainer' -v` on a
  Linux runner. The pattern names the `TestContainer*` functions this
  task adds, and the reviewer reads them in the `-v` output (10 10.8).
