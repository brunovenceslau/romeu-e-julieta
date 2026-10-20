---
id: T151
title: "`gitsafe`: the hardened fast-forward and clone"
milestone: M1
module: gitsafe
fate: new
owns:
  - internal/gitsafe/fetch*.go
  - internal/gitsafe/clone*.go
  - internal/gitsafe/status*.go
  - internal/gitsafe/gitsafe_test.go
  - docs/handoff/pending/gitsafe-s-fast-forward-overwrites-ignored-files-and-runs.md
needs:
  - T051
  - T143
after: []
ask_first:
  - gitsafe
sitting: none
---

# T151 - `gitsafe`: the hardened fast-forward and clone

Goal: harden the two `gitsafe` verbs that write a checkout,
`FastForward` and `Clone`, before `julieta setup` runs in daily use, as
decided on 2026-10-20 (a task on the `gitsafe` surface, with an
approval line). Two security audits of 2026-10-20 found, and the
pending note
[gitsafe's fast-forward overwrites ignored files and runs smudge filters](../../handoff/pending/gitsafe-s-fast-forward-overwrites-ignored-files-and-runs.md)
records, four gaps:

1. `FastForward` (`merge --ff-only`) overwrites an ignored, untracked
   file when the incoming commit adds a tracked file at its path, and
   `Status.Clean()`, the caller's check, does not cover ignored files.
   The fast-forward now refuses it: git's own `--no-overwrite-ignore`
   makes the merge fail and name the paths, and `setup` reports the
   repo as `failed` with that error. Refusing, rather than moving and
   naming the paths, follows the spec: REQ-0399 fast-forwards only a
   clean default branch and reports otherwise, a refused fast-forward
   is a warning, the threat model of 05 counts destroying work, and
   REQ-0316 counts ignored files as work that salvage must recover.
2. The merge's checkout runs `filter.<name>.smudge` and `.process`
   from the clone's or the global config. `FastForward` now lists the
   drivers as `Status` does and runs the merge with each emptied.
3. `filterOverrides` dropped a driver with an empty name (`[filter
   ""]`, key `filter..clean`), which git lists and runs for the
   attribute `filter=` with no value.
4. A sandbox `Clone` checked out HEAD with no filter override. It now
   lists the drivers of the global config from the root directory, as
   it resolves the URL, and clones with each emptied.

One list, `filterNeutral` (`clean`, `smudge` and `process` empty,
`required` false), serves `Status`, `FastForward` and `Clone`.

- Implements: none: the hardening serves REQ-0399 and REQ-0463, which
  T051 and T023 implement.
- Operator: no.
- Acceptance:
  - Each gap is reproduced first by a test that fails before the fix,
    as a user hits it, with real git: an ignored `local.env` (`*.env`
    in `.git/info/exclude`) and an incoming commit that tracks it; a
    smudge driver in the clone's local config, one with a dot in its
    name, a long-running process and two in the sandbox's global
    config, one with an empty name, each required, for the
    fast-forward; a required `[filter ""]` clean driver for `Status`;
    smudge drivers in the sandbox's global config for `Clone`.
  - After the fix the fast-forward fails, names `local.env`, and leaves
    the file and HEAD as they were; no planted filter runs, and the
    files hold the tree's content. Each test keeps its positive
    control: plain git overwrites the file or runs the filter.
  - The argv and environment tests pin the new listing command of
    `Clone` (run from `/`) and the four keys per driver.
  - The tests of `internal/cmd/julieta/setup` pass unchanged.
- Verify: `go test ./internal/gitsafe/... ./internal/cmd/julieta/...`.
