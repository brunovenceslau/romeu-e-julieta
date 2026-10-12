---
id: T024
title: "`gitsafe`: unbundle, verify, fsck and the tree walk"
milestone: M3
module: gitsafe
fate: kept
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
---

# T024 - `gitsafe`: unbundle, verify, fsck and the tree walk

Earlier title: `gitsafe`: bundles, fsck and the tree walk.

Goal: the four verbs of `gitsafe` that only the host calls: bundle
verify and unbundle, fsck, the validated tree walk.

Re-plan (2026-10-12): Kept, in M3: salvage and kit materialization need
it. The runner both modes use and bundle creation belong to T023; this
task's files are the four globs above, so the two file sets stay
disjoint.

- Implements: 05 5.1, 05 5.2 (I27, unit level).
- Operator: no.
- Acceptance:
  - The table of hostile trees of I27 is refused by `WalkTree` without
    relying on fsck: a symlink, a gitlink, the names `.`, `..` and
    `.git` with their case and HFS-ignorable variants, and a case
    collision.
  - `bundle verify` and `unbundle` run under the hardened flags.
- Verify: `go test ./internal/gitsafe/...`.
