---
id: T060
title: "`julieta snapshot`"
milestone: M1
module: salvage
fate: kept
owns:
  - internal/salvage/snapshot*.go
  - internal/cmd/julieta/snapshot/**
  - cmd/julieta/import_snapshot.go
  - e2e/julieta_snapshot_test.go
  - internal/hooks/actions/snapshot*.go
needs:
  - T054
  - T023
after: []
ask_first: []
sitting: none
---

# T060 - `julieta snapshot`

Goal: `julieta snapshot`: the bundle after `post-commit`, `post-rewrite`
and `post-merge`, the two atomic writes, `disabled` on a repo-local
`hooksPath`, the three hook actions one file each.

Re-plan (2026-10-12): Kept. The bundle excludes objects reachable from
`repos[].base`; a race with a commit leaves a valid bundle (a seam, no
sleep). The bundle is created through `gitsafe`'s sandbox mode (T023,
`BundleCreate`), which this task needs.

- Implements: 08 8.4, 04 4.3 (`snapshot`).
- Operator: no.
- Acceptance:
  - The dispatcher calls it after `post-commit`, `post-rewrite` and
    `post-merge`.
  - The bundle holds the local branches, tags, notes, `salvage/*` refs
    and each worktree HEAD minus the objects reachable from
    `repos[].base`.
  - A snapshot that races a commit leaves a valid bundle (10 10.1), with
    the interleaving forced through a test seam and never by sleeping; a
    repo-local `core.hooksPath` is reported as "snapshots disabled".
- Verify: `go test -tags e2e ./e2e/... -run '^TestJulietaSnapshot' -v`.
  The pattern names the `TestJulietaSnapshot*` functions this task adds,
  and the reviewer reads them in the `-v` output (10 10.8).
