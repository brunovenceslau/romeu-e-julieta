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
  - internal/hooks/actions/actions.go
  - internal/gitsafe/reach*.go
  - internal/gitsafe/commondir*.go
  - internal/gitsafe/refs_list*.go
  - internal/gitsafe/refs_parity_test.go
  - internal/gitsafe/export_test.go
  - internal/gitsafe/bundle_of.go
  - internal/gitsafe/gitsafe.go
  - internal/memstore/write/write.go
  - internal/memstore/write/write_test.go
  - docs/spec/02-layouts.md
needs:
  - T054
  - T023
  - T135
  - T153
after: []
ask_first:
  - gitsafe
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
  - With a repo-local `core.hooksPath`, `julieta snapshot` writes
    `disabled: true`. That `julieta setup` writes it, and that `setup`,
    `julieta status` and SessionStart report "snapshots disabled for
    <dir>", belongs to a later task (decided on 2026-10-21).
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

- An input the snapshot cannot read (decided on 2026-10-21): the
  snapshot skips that item, snapshots the rest, and records the skipped
  item as a hook failure of the repo. `gitsafe.ListRefs` returns the
  names it refuses beside the refs it accepts, at most 100, each clipped
  to 128 bytes before it is escaped, and the count of the rest; one
  record over 4 KiB of name fails the list without git's output being
  read further. `actions.Snapshot` returns `ErrSnapshotPartial` naming
  each item, which the dispatcher records (REQ-0409) and `julieta
  snapshot` exits 1 on. The rule, as the reading closest to that
  decision (skip the item, snapshot the rest, lose nothing):
  - a single refused ref name is skipped and the snapshot written;
  - the loss of a namespace (more refs than `ListRefs` lists) or of a
    whole source of reflog commits (the worktree list, a worktree whose
    common git dir, read with `gitsafe.CommonDir` and compared with its
    links resolved, is not the repository's own, a reflog that cannot
    be read, a reflog commit git cannot walk) does not replace an
    existing pair: the earlier pair stays, the failure is recorded, and
    the plan is written only where no earlier pair exists;
  - a partial plan with nothing beyond the bases writes nothing, so the
    earlier `heads.json` and bundle stay;
  - the reflog commits over the cap of 1000 are a recorded skip that
    still writes, or a repository with more would never be snapshot
    again.
  Reopened by an input git can read only in part.
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
  disk is one snapshot's (REQ-0690). The whole snapshot, the wait for
  the lock included, ends after 60 s (`actions.SnapshotTimeout`), and
  the wait honours the context (`write.LockContext`, which tries the
  lock every 10 ms). Reopened by a repository whose snapshot needs
  longer.
- Whether a bundle holds anything is asked before it is made
  (`gitsafe.Beyond`, `git rev-list --objects` up to its first line);
  when the bases reach every object, the snapshot removes the earlier
  bundle in place of the first write and writes `heads.json`, so the
  pair stays one snapshot's and `disabled` is written for a repo with
  no work, a repo with no ref included. No git message is read.
- `heads.json` lists every ref the snapshot read; the bundle holds only
  the refs with an object beyond the bases, since git leaves out a ref
  a base reaches. romeu trusts the bundle's refs (REQ-0689).
- Names: a detached worktree HEAD is
  `refs/salvage-reflog/worktrees/<k>/HEAD` and a commit of its HEAD
  reflog `refs/salvage-reflog/worktrees/<k>/<n>`, with `k` the
  worktree's place in `git worktree list` and `n` the entry's index as
  git numbers it (`<ref>@{n}`), the branches' too. A name already
  taken gets the suffix `-<k>`, the first free, so no commit is lost
  to a name. Reopened by romeu's import, should it need the worktree's
  path.
- Which reflog commits need a name is one question to git
  (`gitsafe.Unreached`, `git rev-list --parents --stdin` with every
  bundled ref and the bases excluded): a commit the walk lists and that
  is no parent of a walked commit is reached by no bundled ref and by
  no other reflog commit. At most 1000 are named per snapshot
  (`salvage.MaxReflogCommits`); the rest is a recorded skip. Measured
  on a repository of 300 branches and a 300-entry reflog of orphan
  commits: 1 min 19.9 s with one `AheadBehind` per commit and tip,
  0.24 s with `Unreached`. When git cannot finish the walk of the
  whole set (a commit whose parent is missing), each commit is asked
  alone and one that fails is a whole-source skip. `Unreached` reads a
  line of up to 1 MiB, a merge of about 16,000 parents; a wider one
  fails the walk and falls to that path. `gitsafe.Beyond` answers true
  for a walk git cannot finish, so the bundle meets the failure and it
  is recorded. Reopened by a snapshot too slow after a commit.
- The copy of the bundle into the memory dir stops when the snapshot's
  context ends (a reader that checks it), and its error path removes
  the temporary file.
- Deferred: the reflog reads cost two git processes per branch and per
  worktree, and the hook can wait up to the whole 60 s. Reopened by a
  repository whose snapshot times out.
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
- Deferred: the snapshot is not bounded by `salvage.capBytes`
  (REQ-0648 is not implemented here). Reopened when romeu's unbundle of
  the snapshot is built.
- Deferred: `salvage.CheckRef` and `gitsafe`'s `unsafeRune` refuse
  the control and format characters and the line and paragraph
  separators, not the whole Default_Ignorable set `termsafe` escapes; a
  test pins that the two ref checks agree. One definition is reopened
  with the readers task or a display bug.
- Deferred: the dispatcher's handler finds the repo of a hook by
  listing the worktrees of each manifest repo, one `git worktree list`
  per repo on each commit outside a repo's own path. Reopened by a
  manifest of many repos.
- The built bundle is judged with `Lstat` and must be a regular file
  before it is opened, and the work directory is made 0700 on each
  run.
- The golden is `testdata/heads.json.golden`: a `.json` file under
  `testdata/` naming `snapshot/v1` is read by `tools/ci schema` as a
  registered format, and 03 3.13 gives `heads.json` none.
