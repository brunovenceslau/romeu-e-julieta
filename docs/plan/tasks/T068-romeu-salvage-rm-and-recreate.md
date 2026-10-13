---
id: T068
title: "`romeu salvage`, `rm` and `recreate`"
milestone: M4
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/salvage/**
  - internal/cmd/romeu/rm/**
  - internal/cmd/romeu/recreate/**
  - internal/cmd/romeu/all/import_salvage.go
  - internal/cmd/romeu/all/import_rm.go
  - internal/cmd/romeu/all/import_recreate.go
  - e2e/scenarios/j6/**
  - e2e/scenarios/j10/**
  - e2e/scenarios/j13/**
  - e2e/salvage_test.go
  - e2e/rm_test.go
  - e2e/recreate_test.go
  - e2e/internal/romeu/salvage_helpers.go
needs:
  - T062
  - T066
after: []
ask_first: []
sitting: none
---

# T068 - `romeu salvage`, `rm` and `recreate`

Goal: `romeu salvage`, `rm` and `recreate`, whole.

Re-plan (2026-10-12): Kept whole. The first uncommitted change lost to a
recreate moves this task ahead of any remaining M3 task (Accepted
risks). Its e2e files are `e2e/salvage_test.go`, `e2e/rm_test.go` and
`e2e/recreate_test.go`, and the helper file it adds to the harness of
T041 is `e2e/internal/romeu/salvage_helpers.go` (rule 1 of
`docs/plan.md`). The I26 case after `adopt` is T050's.

- Implements: REQ-0012 (index), REQ-0052 (01 1.2), REQ-0064 (01 1.4),
  REQ-0074 (01 1.5), REQ-0076 (01 1.5), REQ-0079 (01 1.6), REQ-0080 (01
  1.6), REQ-0083 (01 1.6), REQ-0090 (01 1.6), REQ-0091 (01 1.6),
  REQ-0092 (01 1.6), REQ-0093 (01 1.6), REQ-0094 (01 1.6), REQ-0095 (01
  1.6), REQ-0096 (01 1.6), REQ-0097 (01 1.6), REQ-0098 (01 1.6),
  REQ-0099 (01 1.6), REQ-0100 (01 1.6), REQ-0101 (01 1.6), REQ-0102 (01
  1.6), REQ-0103 (01 1.6), REQ-0104 (01 1.6), REQ-0105 (01 1.6),
  REQ-0106 (01 1.6), REQ-0107 (01 1.6), REQ-0143 (02 2.3), REQ-0144 (02
  2.3), REQ-0171 (03 (opening)), REQ-0223 (03 3.6), REQ-0234 (03 3.8),
  REQ-0235 (03 3.8), REQ-0236 (03 3.8), REQ-0237 (03 3.8), REQ-0238 (03
  3.8), REQ-0249 (03 3.11), REQ-0250 (03 3.11), REQ-0272 (04 4.1),
  REQ-0311 (04 4.2), REQ-0315 (04 4.2), REQ-0316 (04 4.2), REQ-0317 (04
  4.2), REQ-0318 (04 4.2), REQ-0319 (04 4.2), REQ-0320 (04 4.2),
  REQ-0321 (04 4.2), REQ-0348 (04 4.2), REQ-0349 (04 4.2), REQ-0355 (04
  4.2), REQ-0428 (04 4.4), REQ-0430 (04 4.4), REQ-0434 (04 4.4),
  REQ-0436 (04 4.4), REQ-0437 (04 4.4), REQ-0447 (04 4.4), REQ-0448 (04
  4.4), REQ-0449 (04 4.4), REQ-0450 (04 4.4), REQ-0451 (04 4.4),
  REQ-0452 (04 4.4), REQ-0453 (04 4.4), REQ-0454 (04 4.4), REQ-0455 (04
  4.4), REQ-0456 (04 4.4), REQ-0457 (04 4.4), REQ-0486 (05 5.2),
  REQ-0487 (05 5.2), REQ-0501 (05 5.2), REQ-0511 (05 5.2), REQ-0519 (05
  5.2), REQ-0521 (05 5.2), REQ-0522 (05 5.2), REQ-0523 (05 5.2),
  REQ-0524 (05 5.2), REQ-0525 (05 5.2), REQ-0526 (05 5.2), REQ-0527 (05
  5.2), REQ-0528 (05 5.2), REQ-0529 (05 5.2), REQ-0530 (05 5.2),
  REQ-0531 (05 5.2), REQ-0532 (05 5.2), REQ-0533 (05 5.2), REQ-0534 (05
  5.2), REQ-0552 (05 5.4), REQ-0580 (06 6.2), REQ-0585 (06 6.3),
  REQ-0634 (07 7.5), REQ-0683 (08 8.3), REQ-0693 (08 8.5), REQ-0700 (08
  8.5), REQ-0702 (08 8.5), REQ-0703 (08 8.5), REQ-0704 (08 8.5),
  REQ-0711 (08 8.5), REQ-0712 (08 8.5), REQ-0714 (08 8.5), REQ-0715 (08
  8.5), REQ-0716 (08 8.5), REQ-0717 (08 8.5), REQ-0718 (08 8.5),
  REQ-0719 (08 8.5), REQ-0720 (08 8.5), REQ-0721 (08 8.5), REQ-0722 (08
  8.5), REQ-0726 (09 (opening)), REQ-0727 (09 (opening)), REQ-0736 (10
  10.1), REQ-0739 (10 10.1), REQ-0780 (10 10.1), REQ-0784 (10 10.1),
  REQ-1214 (01 1.6), REQ-1215 (01 1.6), REQ-1216 (01 1.6), REQ-1217 (01
  1.6), REQ-1218 (01 1.6), REQ-1219 (01 1.6).
- Operator: no.
- Acceptance:
  - `rm` exits 5 for each case that the `E:` case of I17 opening with
    "`rm` exits 5 for" lists (round 17, R17-07).
  - Each `E:` case of I17 is one test with a title of its own, so the
    cases are cited by title and not by their place in the one list of
    05 5.2 that holds them (round 14, R14-10; round 15, R15-15). 05 5.2
    gives the cases no titles, so each test's title is the opening words
    of its case, and the tests this task adds beyond the cases carry the
    titles quoted below, the `--accept-loss` branch of `RJ-338` as
    "accept-loss names the skipped reasons".
  - Each row of the generation table has its Effect test, by event title
    (01 1.6).
  - `--accept-loss` removes the sandbox after an incomplete salvage;
    `rm` without a `final` handoff of the generation asks on a TTY
    whether to go on (a no exits before `julieta salvage` is called) and
    without a TTY warns `no-final-handoff` and goes on; in both, the
    check runs after the preflight and before the sandbox half (04 4.2,
    08 8.3). The `rm` and `retire` confirmations go to stderr and stdin
    decides the TTY; the `/dev/ptmx` case of `rm` drives stdin and reads
    stderr.
  - A standalone `salvage` runs `julieta salvage` without
    `--stop-agents` and says so; `rm` passes it. A stopped sandbox is
    started for the sandbox half and stopped again. A failed start or
    sandbox half exits 1 naming `--from-host` and the four kinds of work
    it does not recover. `rm` on a terminal asks when no `final` handoff
    exists, and prints the workload volumes salvage does not cover.
  - With the fake sbx failing the create after a complete salvage,
    `recreate` exits 1 with `kit-build-failed`, the generation is
    `closed-removed`, the salvage refs and memory are intact, no sandbox
    exists, and a later `run` creates one (04 4.2, J6 step 5).
  - Preserving a lost generation prints the lost record with its salvage
    ref prefix and the fixed block, and exits 1 with `generation-lost`
    (`RJ-335`); the reasons hold `dirty-at-last-facts:<n>` and
    `stashes-at-last-facts:<n>`; a lost record already written by
    `salvage --from-host` is kept, not duplicated; the next `run`
    creates a new sandbox with a new ULID (J10 step 2). The J10 scenario
    runs both step 3 lines of its `commands.yaml` against the host
    clone, and the `recover/<b>` branch reaches the fixture origin.
  - One test per reason slug of 03 3.11 (`repo-missing`,
    `daemon-heads-missing`, `payload-mismatch`), and the error
    `salvage-id-mismatch` (`RJ-339`, exit 5) from the host half's step
    1; a daemon URL whose scheme or host gitsafe refuses is the skipped
    item `daemon-url-refused`, whose message names sbx.
  - `romeu salvage` prints the secret-bearing sentence of 08 8.5 once
    when the manifest lists ignored files or transcripts, and not
    otherwise.
  - `rm` leaves a closed generation's base refs while a salvage record
    of it exists.
  - The `RJ-338` details name the kind that changed.
  - With a workload that declares a volume, `rm` and `recreate` print,
    escaped, the workload volume list of the A12 record (a fixture
    marked `source: sitting M2` until block O3) and what a recreate
    keeps and loses of them, before the sandbox half runs (06 6.2); of
    the volumes, romeu removes none of its own: `sbx env rm` removes
    what sbx removes, and this task asserts that argv (01 1.2).
  - The fingerprint cases of I17 (round 17, R17-01, R17-02, R17-08,
    R17-09, R17-12):
    - "write during the salvage" starts from a fingerprint that already
      differs when the rerun starts, and its writer changes the tracked
      file between every pair of reads of HEAD and status, through the
      repeat of 08 8.5 step 2, so the worktree is skipped as
      `worktree-changing`: the rerun exits 5 with `RJ-306` and the state
      is `open` (R17-01);
    - one more test, "accept-loss names the skipped reasons", runs the
      branch of `RJ-338` where `--accept-loss` names the rerun's skipped
      reasons: the fingerprint already differs when the rerun starts, a
      writer keeps a tracked file changing through the repeat, and the
      rerun, run with `--accept-loss=worktree-changing`, exits 5 with
      `RJ-338`, the state is `removing`, the new entry records the
      accepted reason, and `sbx env rm` is not reached (reaching it, or
      returning `RJ-306`, turns it red); the case whose salvage is
      complete does not stand for this branch, since it skips no reason
      (R17-02; round 18, R18-A8). The entry's `fingerprint` is that of
      the last read of the skipped worktree (03 3.8): once the writer
      stops, a rerun that finds the worktree as that read left it
      resumes at `sbx env rm` (R18-A4);
    - the cases run for `rm` and for `recreate`, each asserting `RJ-338`
      or `RJ-306` on a rerun on a `removing` generation, and T069 runs
      them for `retire` (R17-08);
    - "kill the salvage" sends SIGTERM to romeu after each step of the
      write order of 01 1.6 from the write of `salvaging` up to, and not
      including, the write of `removing`, one test per step, through the
      signal rule of 04 4.1 and not a test hook (R17-09; round 18,
      R18-A8); an interrupted salvage exec exits 128 plus the signal
      number and the generation is `open` with `sandbox-half-failed`;
    - a fresh fingerprint that cannot be read (the sandbox does not
      start, the daemon is unreachable, `julieta salvage --fingerprint`
      exits non-zero) makes the rerun exit 1 with the state still
      `removing`, no new salvage entry and `sbx env rm` not reached
      (round 18, R18-04);
    - the `excludeIgnored` case edits `salvage.excludeIgnored` through
      `romeu sync`, which promotes the edit (gate 2) and then exits 2
      with `removal-pending` (04 4.2, the `sync` steps), and the test
      asserts both (R17-12).
  - `recreate` is `rm`, then `run`: it salvages, removes, records a new
    generation, and exits 5 where `rm` would.
  - After `salvage`, `refs/heads`, `refs/tags`, `refs/remotes` and
    `HEAD` are unchanged (I5).
  - After a same-name recreate, a fetch with `--prune` and a forced
    refspec leaves the salvage refs, and a second create on one of them
    fails (I6).
  - After `rm`, the romeu-applied egress rules are removed with the
    recorded argv, one applied by a reconcile cut short included (the
    rules `egressApplied` names), and the generation is `closed-removed`
    (J6 step 5).
  - The command runs the compatibility check of 06 6.3 before its first
    julieta call and exits 2 with that check's id on a failure.
  - `salvage --from-host` and the absent-sandbox row of `run` record
    `result: lost`, and after a lost generation is preserved the ref
    `refs/romeu/salvage/<name>/<G>/<id>/<dir>/<b>` exists, which is what
    the J10 step 3 command reads. The J6 and J13 scenario functions land
    here; J6 runs in CI at the hybrid level on the Linux runners, with
    `julieta salvage --stop-agents` forwarded through `sbx env exec` and
    a fixture process with the agent's name standing in for the agent,
    and on the macOS runners the J6 steps that reach no julieta call
    run; J10 runs in CI.
- Verify: `go test -tags e2e ./e2e/... -run
  '^(TestSalvage|TestRm|TestRecreate)' -v`. The pattern names the
  `TestSalvage*` and `TestRm*` and `TestRecreate*` functions this task
  adds, and the reviewer reads them in the `-v` output (10 10.8).
