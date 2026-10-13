---
id: T069
title: "`romeu retire`"
milestone: M4
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/retire/**
  - internal/cmd/romeu/all/import_retire.go
  - e2e/scenarios/j7/**
  - e2e/retire_test.go
needs:
  - T068
  - T048
after: []
ask_first: []
sitting: none
---

# T069 - `romeu retire`

Goal: `romeu retire`.

Re-plan (2026-10-12): Kept as written before the re-plan; its e2e file
is `e2e/retire_test.go` (rule 1 of `docs/plan.md`).

- Implements: REQ-0012 (index), REQ-0048 (01 1.2), REQ-0052 (01 1.2),
  REQ-0056 (01 1.3), REQ-0081 (01 1.6), REQ-0096 (01 1.6), REQ-0099 (01
  1.6), REQ-0105 (01 1.6), REQ-0108 (01 1.6), REQ-0109 (01 1.6),
  REQ-0138 (02 2.3), REQ-0140 (02 2.3), REQ-0141 (02 2.3), REQ-0147 (02
  2.3), REQ-0230 (03 3.8), REQ-0236 (03 3.8), REQ-0272 (04 4.1),
  REQ-0322 (04 4.2), REQ-0323 (04 4.2), REQ-0324 (04 4.2), REQ-0325 (04
  4.2), REQ-0326 (04 4.2), REQ-0447 (04 4.4), REQ-0448 (04 4.4),
  REQ-0449 (04 4.4), REQ-0450 (04 4.4), REQ-0451 (04 4.4), REQ-0452 (04
  4.4), REQ-0453 (04 4.4), REQ-0454 (04 4.4), REQ-0455 (04 4.4),
  REQ-0456 (04 4.4), REQ-0457 (04 4.4), REQ-0501 (05 5.2), REQ-0521 (05
  5.2), REQ-0522 (05 5.2), REQ-0523 (05 5.2), REQ-0524 (05 5.2),
  REQ-0525 (05 5.2), REQ-0526 (05 5.2), REQ-0527 (05 5.2), REQ-0528 (05
  5.2), REQ-0529 (05 5.2), REQ-0530 (05 5.2), REQ-0531 (05 5.2),
  REQ-0532 (05 5.2), REQ-0533 (05 5.2), REQ-0534 (05 5.2), REQ-0580 (06
  6.2), REQ-0720 (08 8.5), REQ-0724 (09 (opening)), REQ-0736 (10 10.1).
- Operator: no.
- Acceptance:
  - The J7 scenario passes in CI: `retire` refuses a project the spec
    still lists unless `--force`; runs `rm` if needed and exits 5 before
    any move when the inner salvage is incomplete and `--accept-loss`
    does not name its reasons; lists unpushed and salvage-only work per
    repo and asks on a terminal; exits 5 on a declined confirmation, or
    without a terminal unless `--accept-loss` is given; says whether the
    workload volumes are left (06 6.2); moves `<name>-env/` to
    `.attic/<name>/<ts>/`, drops the project from the workspace files
    and moves its state to the state attic. The confirmation goes to
    stderr and stdin decides the TTY; the `/dev/ptmx` case of `retire`
    drives stdin and reads stderr.
  - Nothing is deleted; `.attic/` is created with mode 0700.
  - Retire fault injection: romeu is killed after each of the three
    steps of 01 1.6 (Retire), one test per step; the next `status`,
    `sync` and `retire` each finish the remaining steps before anything
    else, and `sync` never recreates `<name>-env/` for a record with
    `retiring` (03 3.8).
  - After `retire`, `status` and `doctor` report nothing about
    `.attic/<name>/` or the state attic (J7 step 4).
  - On a `removing` generation with its sandbox present, `retire` runs
    the fingerprint cases of I17 that T068 runs for `rm`: an equal
    fingerprint resumes at `sbx env rm`, and one that differs salvages
    again and ends as `RJ-338` (04 4.4) says; on an exit with `RJ-338`
    or `RJ-306` nothing is moved (the tree, the workspace files, the
    state), and after the resume the move of J7 completes (round 17,
    R17-08; round 18, R18-05, R18-A9).
- Verify: `go test -tags e2e ./e2e/... -run '^TestRetire' -v`. The
  pattern names the `TestRetire*` functions this task adds, and the
  reviewer reads them in the `-v` output (10 10.8).
