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
  - cmd/romeu/import_salvage.go
  - cmd/romeu/import_rm.go
  - cmd/romeu/import_recreate.go
  - e2e/scenarios/j6/**
  - e2e/scenarios/j10/**
  - e2e/scenarios/j13/**
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
risks).

- Implements: 04 4.2 (`salvage`, `rm`, `recreate`), 08 8.5, 01 1.6, 09
  (J6, J10, J13), 05 5.2 (I5, I6, I17, I26).
- Operator: no.
- Acceptance:
  - `rm` exits 5 for each case that the `E:` case of I17 opening with
    "`rm` exits 5 for" lists (round 17, R17-07).
  - Each `E:` case of I17 is one test with a title of its own, so the
    cases are cited by title and not by their place in the one cell
    of 05 5.2 that lists them (round 14, R14-10; round 15, R15-15).
    05 5.2 gives the cases no titles, so each test's title is the
    opening words of its case, and the tests this task adds beyond the
    cases carry the titles quoted below, the `--accept-loss` branch of
    `RJ-338` as "accept-loss names the skipped reasons".
  - `--accept-loss` removes the sandbox after an incomplete salvage;
    `rm` warns when the generation has no `final` handoff.
  - The fingerprint cases of I17 (round 17, R17-01, R17-02, R17-08,
    R17-09, R17-12):
    - "write during the salvage" starts from a fingerprint that already
      differs when the rerun starts, and its writer changes the tracked
      file between every pair of reads of HEAD and status, through the
      repeat of 08 8.5 step 2, so the worktree is skipped as
      `worktree-changing`: the rerun exits 5 with `RJ-306` and the state
      is `open` (R17-01);
    - one more test, "accept-loss names the skipped reasons", runs the
      branch of `RJ-338` where `--accept-loss` names the rerun's
      skipped reasons: the fingerprint already
      differs when the rerun starts, a writer keeps a tracked file
      changing through the repeat, and the rerun, run with
      `--accept-loss=worktree-changing`, exits 5 with `RJ-338`, the
      state is `removing`, the new entry records the accepted reason,
      and `sbx env rm` is not reached (reaching it, or returning
      `RJ-306`, turns it red); the case whose salvage is complete does
      not stand for this branch, since it skips no reason (R17-02;
      round 18, R18-A8). The entry's `fingerprint` is that of the last
      read of the skipped worktree (03 3.8): once the writer stops, a
      rerun that finds the worktree as that read left it resumes at
      `sbx env rm` (R18-A4);
    - the cases run for `rm` and for `recreate`, each asserting
      `RJ-338` or `RJ-306` on a rerun on a `removing` generation, and
      T069 runs them for `retire` (R17-08);
    - "kill the salvage" kills romeu after each step of the write
      order of 01 1.6 from the write of `salvaging` up to, and not
      including, the write of `removing`, one test per step (R17-09;
      round 18, R18-A8);
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
  - After `adopt`, `salvage` and `rm` succeed on the adopted sandbox
    (I26).
  - After `rm`, the romeu-applied egress rules are removed with the
    recorded argv and the generation is `closed-removed` (J6 step 5).
  - `salvage --from-host` and the absent-sandbox row of `run` record
    `result: lost`, and after a lost generation is preserved the ref
    `refs/romeu/salvage/<name>/<G>/<id>/<dir>/<b>` exists, which is what
    the J10 step 3 command reads. The J6 and J13 scenario functions land
    here, and J10 runs in CI.
- Verify: `go test -tags e2e ./e2e/... -run
  '^(TestSalvage|TestRm|TestRecreate)' -v`. The pattern names the
  `TestSalvage*` and `TestRm*` and `TestRecreate*` functions this task
  adds, and the reviewer reads them in the `-v` output (10 10.8).
