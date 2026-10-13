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
  - .github/workflows/ci.yml
  - tools/ci/internal/workflows/**
needs:
  - T009
  - T012
after:
  - T105
ask_first:
  - checks
  - approvals
  - dependencies
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
in `e2e/kit_<name>_test.go`, owned by that task (rule 1 of
`docs/plan.md`; 10 10.1), so no two tasks append to one test file. It
owns `.github/workflows/ci.yml` and the `workflows` check's action-input
list for the steps it adds, after T105, which moves that check.

- Implements: REQ-0120 (02 2.1), REQ-0131 (02 2.1), REQ-0740 (10 10.1),
  REQ-0744 (10 10.1), REQ-0745 (10 10.1), REQ-0746 (10 10.1), REQ-0798
  (10 10.1), REQ-0805 (10 10.2), REQ-0812 (10 10.2), REQ-0832 (10 10.2),
  REQ-0837 (10 10.2).
- Operator: no.
- Acceptance:
  - The container e2e runs `julieta version --json` in the workload's
    Debian base, pulled by the digest of the pin file, on `ubuntu-26.04`
    and `ubuntu-26.04-arm`, native.
  - A pull that fails on transport (DNS, a timeout, a 5xx) is reported
    as an infrastructure error, the way 10 10.1 treats a failed mise
    download: a red job with its own exit code, never a skip, and a
    fault test makes the pull fail that way and asserts the code. A
    manifest digest that differs from the pin is a test failure, always:
    it is the one signal that a pinned artifact was substituted, and a
    security gate is hardened, not retried.
  - An unreachable Docker daemon, a failed herdr download and a failed
    mise download each end the job with the infrastructure exit status,
    a fault test each (10 10.1).
  - `ci.yml` restores the container e2e's mise cache with
    `actions/cache` pinned by a 40-hex SHA, keyed by the sha256 of the
    fixture `mise.lock`; `tools/ci workflows` lists the inputs of
    `actions/cache`, and a fixture with an unlisted input fails.
  - Every `jdx/mise-action` step of `ci.yml` writes
    `persist_github_token: false`, which the `workflows` grammar
    requires on every mise action step, with a fixture that lacks it.
  - `ci.yml` sets a `concurrency` group per pull request ref with
    `cancel-in-progress`, so an edit of a pull request's body does not
    queue a second full run; the grammar admits the two keys with a
    fixture.
  - `tools/ci all` gains the `e2e` step and the "build the tagged
    suites" step of 10 10.2 here, each registered by file (rule 4 of
    `docs/plan.md`); until T103 lands `tools/release build --dry-run`,
    the `e2e` step runs only the julieta cases, which need no romeu
    build; from here on every `-tags e2e` Verify line of a later task
    runs in CI.
  - The steps each runner class runs are one table in `tools/ci` with a
    unit test of its own; `tools/ci all` selects the step from the OS
    and architecture it runs on (10 10.2). A runner-selected step runs
    `go test -json` through a reader that counts the Test events and
    fails the step at zero tests; a fixture package with no test turns
    it red.
- Verify: `go test -tags e2e ./e2e/... -run '^TestContainer' -v` on a
  Linux runner. The pattern names the `TestContainer*` functions this
  task adds, and the reviewer reads them in the `-v` output (10 10.8).
