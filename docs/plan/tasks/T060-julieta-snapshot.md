---
id: T060
title: "`julieta snapshot`"
milestone: M1
module: salvage
fate: kept
owns:
  - internal/salvage/snapshot*.go
  - internal/cmd/julieta/snapshot/**
  - internal/cmd/julieta/all/import_snapshot.go
  - e2e/julieta_snapshot_test.go
  - internal/hooks/actions/snapshot*.go
needs:
  - T054
  - T023
  - T135
  - T153
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
`BundleCreate`), which this task needs; the manifest type comes from
T135.

Re-plan (2026-10-21): Kept, changed. On T153 (#224), the bundle is
built with `gitsafe`'s `ListRefs` (the local branches, tags, notes and
the current generation's `refs/salvage/<generation>/`), `Reflog` (each
branch's and each worktree's HEAD reflog), `Worktrees` and `BundleOf`,
in place of a direct `BundleCreate`; `BundleOf` calls `BundleCreate`
itself, which is why T023 stays in `needs`. `BundleOf` writes no ref
into the user's repository and returns `ErrNoRefs` when it is given no
pair, which a repository with no ref and no worktree HEAD gives. T060
names each worktree HEAD and each reflog-only commit under the
namespaces `BundleOf` accepts. Two questions T153 left to this task:
whether a snapshot skips and records an input `gitsafe` refuses whole
(a ref name `ListRefs` refuses, more refs than `ListRefs` lists, a
reflog `Reflog` refuses), which today fails the whole snapshot; and the
removal of a `.gitsafe-bundle-*` directory that a killed process left
in the snapshot directory, which callers otherwise ignore. Its
`disabled` acceptance item reaches `setup`, `status` and the
SessionStart print, outside its owns: settled when the task is built,
by an `after` edge or a narrower item.

- Implements: REQ-0157 (02 2.6), REQ-0168 (03 (opening)), REQ-0222 (03
  3.6), REQ-0239 (03 3.8), REQ-0261 (03 3.13), REQ-0278 (04 4.1),
  REQ-0409 (04 4.3), REQ-0416 (04 4.3), REQ-0686 (08 8.4), REQ-0688 (08
  8.4), REQ-0689 (08 8.4), REQ-0690 (08 8.4), REQ-0691 (08 8.4),
  REQ-0692 (08 8.4), REQ-0741 (10 10.1), REQ-0778 (10 10.1), REQ-0779
  (10 10.1).
- Operator: no.
- Acceptance:
  - The dispatcher calls it after `post-commit`, `post-rewrite` and
    `post-merge`.
  - The bundle holds the local branches, tags, notes, the current
    generation's `refs/salvage/<generation>/*` refs and no earlier
    generation's, each worktree HEAD, and each reflog-only commit as
    `refs/salvage-reflog/<branch>/<n>`, minus the objects reachable from
    `repos[].base`; one bundle is kept.
  - `heads.bundle` is renamed into place before `heads.json` (`schema:
    snapshot/v1`, ref -> SHA, `createdAt`, `disabled`); a kill between
    the two leaves the previous `heads.json` and a valid bundle.
    `julieta snapshot` writes `snapshot/heads.json` with a golden;
    romeu's reader decodes it strictly.
  - On a forced write failure the previous bundle stays, no `.tmp-` file
    of this run remains, the failure is recorded as a hook failure with
    its repo, and the repo's own `post-commit` still runs.
  - With a repo-local `core.hooksPath`, `julieta setup` writes
    `disabled: true`, and `setup`, `julieta status` and SessionStart
    report "snapshots disabled for <dir>".
  - A snapshot that races a commit leaves a valid bundle (10 10.1), with
    the interleaving forced through a test seam and never by sleeping;
    two snapshots of one repo at once are serialized by the store lock
    of that repo's memory dir and leave one snapshot's pair, and the
    test asserts the pair, not a review checkout.
  - The package takes the clock as an injected value, and its tests fix
    it (10 10.4).
- Verify: `go test -tags e2e ./e2e/... -run '^TestJulietaSnapshot' -v`.
  The pattern names the `TestJulietaSnapshot*` functions this task adds,
  and the reviewer reads them in the `-v` output (10 10.8).
