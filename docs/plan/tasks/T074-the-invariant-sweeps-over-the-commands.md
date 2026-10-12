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

Re-plan (2026-10-12): Kept as written before the re-plan.

- Implements: 05 5.2 (I2, I3, I4, I7, I14, I15, I16, I25).
- Operator: no.
- Acceptance:
  - After each romeu command, each clone's `.git/` equals what it was,
    by the comparison of I2.
  - With the hostile configuration of I3 planted in the repos, in a
    global config and in the environment, `init`, `sync`, `run`, `pull`,
    `salvage`, `rm`, `retire`, `status` and `doctor` leave no marker
    file.
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
  - No e2e run reaches the fake `sbx`'s exit 99 for `--auto-approve`
    (I4) or for a secret value form (I25).
- Verify: `go test -tags e2e ./e2e/... -run '^TestSweeps' -v`. The
  pattern names the `TestSweeps*` functions this task adds, and the
  reviewer reads them in the `-v` output (10 10.8).
