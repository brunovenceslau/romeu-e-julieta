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

Re-plan (2026-10-12): Kept as written before the re-plan. It owns
`internal/cli/errorids_exempt.yaml`, which T008's `owns` excludes.

- Implements: REQ-0077 (01 1.5), REQ-0081 (01 1.6), REQ-0087 (01 1.6),
  REQ-0277 (04 4.1), REQ-0278 (04 4.1), REQ-0279 (04 4.1), REQ-0281 (04
  4.1), REQ-0283 (04 4.1), REQ-0488 (05 5.2), REQ-0521 (05 5.2),
  REQ-0522 (05 5.2), REQ-0523 (05 5.2), REQ-0524 (05 5.2), REQ-0525 (05
  5.2), REQ-0526 (05 5.2), REQ-0527 (05 5.2), REQ-0528 (05 5.2),
  REQ-0529 (05 5.2), REQ-0530 (05 5.2), REQ-0531 (05 5.2), REQ-0532 (05
  5.2), REQ-0533 (05 5.2), REQ-0534 (05 5.2), REQ-0737 (10 10.1),
  REQ-0741 (10 10.1), REQ-0775 (10 10.1), REQ-0776 (10 10.1), REQ-0779
  (10 10.1), REQ-0782 (10 10.1), REQ-0783 (10 10.1), REQ-0785 (10 10.1),
  REQ-0787 (10 10.1), REQ-0788 (10 10.1), REQ-0792 (10 10.1), REQ-0794
  (10 10.1), REQ-0796 (10 10.1), REQ-0797 (10 10.1), REQ-0799 (10 10.1),
  REQ-1109 (12 12.3).
- Operator: no.
- Acceptance:
  - The idempotency table T043 started has a row for each converging and
    each appending command of 04 4.1, romeu's and julieta's (`setup`,
    `install`, `lock`, `memory add`, `handoff write` and `snapshot`, at
    the container level); a test fails a command of the command
    definitions that has no row; the row for `stop` runs the second
    `stop` against a running fixture.
  - Two romeu processes: the second exits 1 with `RJ-101`. The
    Concurrency case "two snapshots of one repo at once" asserts the one
    pair the store lock leaves.
  - Promotion fault injection: after a kill at each step, each **P**
    command finishes the promotion or exits 4, and a re-sync converges
    (I7); a candidate that no longer verifies after the kill restores
    the previous digests and exits 4 with
    `interrupted-promotion-abandoned`, never drift.
  - A hook kills romeu after each step of the removal half of `rm`,
    `recreate` and `retire`, beyond the salvage steps the
    kill-the-salvage tests of T068 and T069 cover, and the next command
    converges (I17).
  - Each **P** command runs with the registry and the origins
    unreachable after `sync`, and succeeds.
  - The file `internal/cli/errorids_exempt.yaml` holds one purpose: the
    ids of 04 4.4 whose raiser is a deferred or left-v1 task, each with
    that task's id and a reason; the task that lands the raiser removes
    its ids (T056: `RJ-319`, `RJ-320`). No other list is exempt.
  - The every-id-asserted meta-test reads every `_test.go` file of the
    module with `go/ast`, under any build tag, collects the id each call
    of the helper of `internal/cli`'s test-support package names, and
    fails each non-retired id of 04 4.4 that no call names and that
    `internal/cli/errorids_exempt.yaml` does not list with its reason
    and task id (an id measurable only on a real host, as
    `kit-build-failed`, B2, among them).
- Verify: `go test -tags e2e ./e2e/... -run '^TestMeta' -v`. The pattern
  names the `TestMeta*` functions this task adds, and the reviewer reads
  them in the `-v` output (10 10.8).
