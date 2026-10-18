---
id: T024
title: "`gitsafe`: unbundle, verify, fsck and the tree walk"
milestone: M3
module: gitsafe
fate: merged
owns:
  - internal/gitsafe/unbundle*.go
  - internal/gitsafe/verify*.go
  - internal/gitsafe/fsck*.go
  - internal/gitsafe/walk*.go
needs:
  - T023
after: []
ask_first:
  - gitsafe
sitting: none
merged: 2026-10-18
pr: 139
---

# T024 - `gitsafe`: unbundle, verify, fsck and the tree walk

Merged on 2026-10-18 in pull request #139. The text below is the one
written at the re-plan.

Earlier title: `gitsafe`: bundles, fsck and the tree walk.

Goal: the four verbs of `gitsafe` that only the host calls: bundle
verify and unbundle, fsck, the validated tree walk.

Re-plan (2026-10-12): Kept, in M3: salvage and kit materialization need
it. The runner both modes use and bundle creation belong to T023; this
task's files are the four globs above, so the two file sets stay
disjoint. The unit table of the I27 hostile trees is here; the E-level
cases of T044 ride in T043.

- Implements: REQ-0050 (01 1.2), REQ-0053 (01 1.3), REQ-0465 (05 5.1),
  REQ-0512 (05 5.2).
- Operator: no.
- Acceptance:
  - The table of hostile trees of I27 is refused by `WalkTree` without
    relying on fsck: a symlink, a gitlink, the names `.`, `..` and
    `.git` with their case and HFS-ignorable variants, and a case
    collision.
  - `bundle verify` and `unbundle` run under the hardened flags.
- Verify: `go test ./internal/gitsafe/...`.
