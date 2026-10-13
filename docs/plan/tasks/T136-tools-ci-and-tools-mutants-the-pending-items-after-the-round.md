---
id: T136
title: "`tools/ci` and `tools/mutants`: the pending items after the round caps"
milestone: M6
module: ci-release
fate: deferred
owns: []
needs: []
after:
  - T105
  - T137
ask_first:
  - checks
sitting: none
---

# T136 - `tools/ci` and `tools/mutants`: the pending items after the round caps

Goal: The known defects and pending items of the kept `tools/ci` and
`tools/mutants`, each with the id of the host reminder it came from, so
the host store can be closed.

Re-plan (2026-10-12, stage B): Deferred; the plan's "the hardening is
frozen" holds, and the task reopens on a real escape shown by a user or
a run, or at the first release candidate. Until then `owns` is empty;
when it reopens, `owns` is `tools/ci/**` and `tools/mutants/**`, ordered
after T105 and T137.

- Implements: none (every item restates a rule of 10 10.2 or 10 10.6
  that a live task already cites; the ids are those tasks').
- Operator: no
- Acceptance, one line per item, each with its test:
  1. `git.FetchHint` says `git cat-file -e <oid>`, which lazy-fetches
     silently, instead of `-p`, which prints the blob; the three hint
     assertions follow; `headTree`'s hint says more blobs may follow or
     fetches the tree (9e37b140, items 1 and 2).
  2. `pushed.Readings` wraps the `diff-tree -p` failure of a partial
     clone with the fetch hint instead of failing closed with git's
     status 128 (9e37b140, item 7).
  3. `blobAt` and `fileIn` accept only mode 100644 or 100755 for
     `tools/ci/denylist.yaml`, so a symlink is refused (9e37b140, item
     3; security Info).
  4. A default branch with no denylist is named in the pushed-range
     verdict instead of an empty list read silently (9e37b140, item 4;
     security Info).
  5. The wording of the missing-blob errors is one ("the object store is
     missing it" against "lacks its blob"); `blobAt`'s errors pass the
     path through `git.Printable` (9e37b140, items 8 and 9);
     `TestBlobAt` gains `blobAt(head, "tools/")` found=false (item 5);
     `recordGit`'s serial rule is a comment only (item 6); `hookGitEnv`
     gives git the parent's PWD, recorded (item 10).
  6. `hookGitEnv` drops every `GIT_CONFIG_*` variable, so in tests the
     tool's git reads the runner's system gitconfig: tests point
     `GIT_CONFIG_SYSTEM` at an empty fixture file; `NewBare(t)` asserts
     `receive.autoGC=false`; `TestFixturesRunNoBackgroundMaintenance`
     prints the `child_start` lines only on failure (fe9c2862, items 1
     to 3).
  7. `tools/mutants` catches SIGQUIT, writes the goroutine dump with
     `runtime/pprof`, removes the clone and exits 131, or its package
     doc says why not (70e30de0, item 1; security Low).
  8. `tools/mutants`: a test fails when the synchronous group kill
     before `signal.Stop` is removed, or the reopening condition is
     recorded (`goCmd` stops using `exec.CommandContext`) (70e30de0,
     item 2; 6c4a8113); `group.combinedOutput` stops repeating
     `procgroup.CombinedOutput`'s after-return kill (item 3);
     `TestGroupKill` spawns a grandchild, `TestRunSignalledInProcess`
     carries its non-parallel note, the uncovered error branches of
     `newRunner`, `close` and `warnDirty` are covered (item 5);
     `packages()` runs one `go list -find` for all mutants (item 6);
     `main.go:51` fits 80 columns (acca6c31, item 4).
  9. `procgroup_unix_test.go` asserts the command's `*exec.ExitError`
     carries SIGKILL rather than `ctx.Err()` (acca6c31, item 1); the
     package doc of `tools/mutants` moves "A test earns trust" right
     after the synopsis (item 2); `main_test.go:512`'s regexp allows a
     minute or notes the assumption (item 3); a shared process
     test-helper package once a third consumer of `TestHelperProcess`,
     `killAtEnd` and `gone` appears (item 5).
  10. The two stale citations of `tools/mutants/main.go` name the
      deferred row of the plan on `tools/ci mutate` and the task file
      that holds T005's table.
  11. `names.Match` hashes each window once per distinct first-segment
      length and looks the digest up in a set keyed by length, one pass
      per line whatever the entry count; reopened earlier when `hygiene`
      takes over 5 s in `fast` or the denylist holds more than 5 entries
      (the measured 1.2 s for 4.8 MB and 2 entries at 1e63d6a).
  12. The parallel-run flake (a `fast` run failing with a bare `exit
      status 1` while another `all` ran on the same host) is reproduced
      with two parallel runs and the failing step captured (2b55a042;
      the OOM lead of 65d90c10 compared with the failing push time).
  13. A mise shim under a non-default `MISE_DATA_DIR` gets the specific
      hint, not the generic one (65d90c10, item 6); the hostile
      shim-path subtest kills the echo-shim mutant (item 12); the
      other-go hint line fits the terminal (item 13); the `t.Cleanup`
      group SIGKILL runs only on a failing run (item 14);
      `main_test.go:214` fits 80 columns (item 11).
  14. Tests for the seven surviving mutants of T002 (55889487).
  15. `TestMainSecondSignal` hoists its `time.After(10s)` before the
      loop; the `killAtEnd` comment says what `Getpgid` proves; the
      `TestFinish` doc names the table's row count (69725ae0, items 2 to
      4).
- Verify: `go run ./tools/ci all` exits 0; `go run ./tools/mutants` over
  the hygiene package reports no new survivor.
