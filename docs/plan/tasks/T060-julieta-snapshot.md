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

Decisions taken while building (2026-10-21), each with what reopens
it:

- An input `gitsafe` refuses whole (decided on 2026-10-21): the
  snapshot skips that item, snapshots the rest, and records the skipped
  item as a hook failure of the repo. `salvage.Pairs` lists each
  namespace with its own `ListRefs` call, so a refused tag name loses
  `refs/tags/` and keeps the branches; a refused `Worktrees` or
  `Reflog` loses that input only. `actions.Snapshot` writes the pair
  and returns `ErrSnapshotPartial` naming each item, which the
  dispatcher records (REQ-0409) and `julieta snapshot` exits 1 on.
  Reopened by a `gitsafe` verb that lists the refs it accepts and
  reports the rest.
- The `.gitsafe-bundle-*` leftover: the bundle is built in
  `$HOME/.local/state/julieta/snapshot/<dir>/`, never in the memory
  dir, whose layout check (08 8.1) reads a dot directory as a finding
  that `romeu sync` refuses. Each snapshot, under the store lock,
  removes the `.gitsafe-bundle-*` and `heads-*` names a killed snapshot
  left there; the bundle is then copied into `snapshot/` through a
  `.tmp-<ULID>` file, which `julieta setup` stays the cleanup of
  (REQ-0691). Reopened by a memory dir on the file system of julieta
  state, where a rename would do.
- The copy is streamed by `copyAtomic` (`internal/hooks/actions/
  snapshot.go`), which follows `write.AtomicWrite` step for step:
  `AtomicWrite` takes a `[]byte`, and a bundle may reach the salvage
  cap. Reopened by `internal/memstore/write` gaining a reader form,
  which `copyAtomic` then calls.
- The store lock is held from before the read of the refs to after
  the last write, so the work directory has one user and the pair on
  disk is one snapshot's (REQ-0690). Reopened by a snapshot slow enough
  to hold `julieta memory add` back.
- When the bases reach every ref, git refuses an empty bundle; the
  snapshot then removes the earlier bundle in place of the first write
  and writes `heads.json`, so the pair stays one snapshot's and
  `disabled` is written for a repo with no work. A repo with no ref is
  the same case (`gitsafe.ErrNoRefs`). The refusal is read from git's
  message under `LANG=C`. Reopened by a `gitsafe` sentinel for it.
- `heads.json` lists every ref the snapshot read; the bundle holds only
  the refs with an object beyond the bases, since git leaves out a ref
  a base reaches. romeu trusts the bundle's refs (REQ-0689).
- Names: a detached worktree HEAD is
  `refs/salvage-reflog/worktrees/<k>/HEAD` and a commit of its HEAD
  reflog `refs/salvage-reflog/worktrees/<k>/<n>`, with `k` the
  worktree's place in `git worktree list` and `n` the entry's index as
  git numbers it (`<ref>@{n}`), the branches' too. A name a branch
  already takes is skipped and recorded. Reopened by romeu's import,
  should it need the worktree's path.
- Whether a bundled ref reaches a reflog commit is asked one ref at a
  time with `AheadBehind`: about 0.6 s for a branch of 300 commits
  here. Reopened by a snapshot too slow after a commit, with a
  `gitsafe` verb that answers for every commit in one `rev-list`.
- `disabled` is what `julieta snapshot` reads itself: the repo's own
  config sets `core.hooksPath`, as `julieta status` reads it. The
  `setup`, `status` and SessionStart part of the acceptance row is a
  task of its own after T060 (decided on 2026-10-21).
- The snapshot handler of the dispatcher is set by
  `internal/cmd/julieta/snapshot` with `actions.HandleSnapshot`, since
  `internal/hooks/actions` cannot import a package that imports
  `internal/cmd/julieta/setup`; its clock is `time.Now`, a variable the
  tests replace. A manifest repo is found by its path or by one of its
  worktrees; a repo of no manifest takes no snapshot and records
  nothing. Reopened by an `actions.Input` that carries the clock.
- The snapshot is not bounded by `salvage.capBytes` (REQ-0648 is not
  implemented here). Reopened by the task that implements REQ-0648.
- The golden is `testdata/heads.json.golden`: a `.json` file under
  `testdata/` naming `snapshot/v1` is read by `tools/ci schema` as a
  registered format, and 03 3.13 gives `heads.json` none.
