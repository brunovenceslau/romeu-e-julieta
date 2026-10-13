---
id: T074
title: "The invariant sweeps over the commands"
milestone: M4
module: romeu-cli
fate: kept
owns:
  - e2e/sweeps_*_test.go
  - e2e/internal/sweeps/**
needs:
  - T073
after: []
ask_first: []
sitting: none
---

# T074 - The invariant sweeps over the commands

Goal: The invariant sweeps over the commands, one file per sweep.

Re-plan (2026-10-12): Kept as written before the re-plan. The I14
filesystem diff is the E-level case here; the unit-level case is
`doctor`'s trust row (T049).

- Implements: REQ-0001 (index), REQ-0026 (index), REQ-0027 (index),
  REQ-0029 (index), REQ-0047 (01 1.2), REQ-0061 (01 1.4), REQ-0062 (01
  1.4), REQ-0063 (01 1.4), REQ-0065 (01 1.4), REQ-0076 (01 1.5),
  REQ-0078 (01 1.5), REQ-0147 (02 2.3), REQ-0308 (04 4.2), REQ-0365 (04
  4.2), REQ-0458 (05 5.1), REQ-0463 (05 5.1), REQ-0471 (05 5.1),
  REQ-0476 (05 5.2), REQ-0479 (05 5.2), REQ-0483 (05 5.2), REQ-0484 (05
  5.2), REQ-0485 (05 5.2), REQ-0488 (05 5.2), REQ-0497 (05 5.2),
  REQ-0498 (05 5.2), REQ-0499 (05 5.2), REQ-0509 (05 5.2), REQ-0554 (05
  5.4), REQ-0737 (10 10.1), REQ-0751 (10 10.1).
- Operator: no.
- Acceptance:
  - After each romeu command, each clone's `.git/` equals what it was,
    by the comparison of I2.
  - With the hostile configuration of I3 planted in the repos, in a
    global config and in the environment, `init`, `sync`, `run`, `pull`,
    `salvage`, `rm`, `retire`, `status` and `doctor` leave no marker
    file.
  - Each absence sweep (I2, I3, I14) runs once against unhardened git or
    under the guard's pass-through and fails when the marker, the `.git`
    diff or the trust write does not appear (the positive control of 05
    5.2).
  - A filesystem diff over a temp `HOME` after each command shows no VS
    Code trust or settings write (I14) and no deletion outside what
    romeu owns (I16).
  - A hand edit of a live derived file makes `sync` and each **P**
    command exit 4 (I15).
  - Each gate 2 change and each gate 1 change makes `run` exit 3 before
    a mutating call, and `rm`, `retire` and `recreate` exit 3 before
    their removal half, after their salvage half ran; `stop`, `salvage`,
    `salvage --from-host` and `pull` print the failure and go on, and
    widen nothing (01 1.5; I7).
  - With a step-5 failure planted, `stop`, `salvage`, `salvage
    --from-host` and `pull` print it and go on; with drift, an abandoned
    interrupted promotion or an unknown sandbox they stop (01 1.5).
  - Each **P** command prints `sbx-untested` on an sbx outside the
    tested window and goes on.
  - No e2e run reaches the fake `sbx`'s exit 99 for `--auto-approve`
    (I4) or for a secret value form (I25).
- Verify: `go test -tags e2e ./e2e/... -run '^TestSweeps' -v`. The
  pattern names the `TestSweeps*` functions this task adds, and the
  reviewer reads them in the `-v` output (10 10.8).
