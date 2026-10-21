---
id: T153
title: "`gitsafe`: the snapshot bundle, the global hooks path and the dirty count"
milestone: M1
module: gitsafe
fate: new
owns:
  - internal/gitsafe/refs_list*.go
  - internal/gitsafe/reflog*.go
  - internal/gitsafe/bundle*.go
  - internal/gitsafe/global*.go
  - internal/gitsafe/status*.go
  - internal/gitsafe/gitsafe_test.go
  - internal/gitsafe/helper_test.go
  - internal/cmd/julieta/setup/hooks_path*.go
  - internal/cmd/julieta/handoff/facts*.go
  - docs/spec/04-cli.md
needs:
  - T054
  - T059
  - T151
after: []
ask_first:
  - gitsafe
  - spec
sitting: none
---

# T153 - `gitsafe`: the snapshot bundle, the global hooks path and the dirty count

Goal: the `gitsafe` verbs that three merged or planned callers wait
for, in one task on the `gitsafe` surface, as decided on 2026-10-20
(one task for the two pending notes below, approved for planning):

1. The snapshot bundle of REQ-0688, which T060 (`julieta snapshot`)
   cannot build today ("gitsafe verbs for the snapshot bundle",
   01m5d7v42kv79hf42g15ffgk59). `Branches` lists only `refs/heads`, no
   verb reads a reflog, `BundleCreate` takes only refs that exist,
   `CreateRef` is create-only and limited to `refs/salvage/` in sandbox
   mode, and git refuses a bundle of a raw object id ("Refusing to
   create empty bundle", git 2.53.0). This task adds:
   - `ListRefs`: the refs under a list of namespaces the caller names
     from an allowlist (`refs/heads/`, `refs/tags/`, `refs/notes/`,
     `refs/salvage/<generation>/`), with their object ids, bounded like
     `Branches`.
   - `Reflog`: the entries of one ref's reflog, oldest first, as object
     ids, bounded; a ref with no reflog is an empty list.
   - `BundleOf`: a bundle of name and object id pairs, minus the bases,
     built without writing a ref into the user's repository: in a
     temporary bare repository under the caller's directory that borrows
     the objects through `objects/info/alternates`, which gets the refs
     and runs `git bundle create`, and is removed on every path. No pair
     is an empty bundle, reported as its own error the caller can tell
     apart, not git's message.
2. The global `core.hooksPath` of REQ-0397, so git calls the dispatcher
   T054 installs ("gitsafe verbs for the global hooks path and the dirty
   count", 01m5d7v43hd4hsw60e4bhek3pw): `SetGlobalHooksPath`, sandbox
   mode only, writes the key with `git config --global`; host mode
   refuses it (I2). T054's `installDispatcher` calls it, which closes
   its acceptance row 3. julieta's own git calls keep
   `-c core.hooksPath=/dev/null`, so the global key does not make them
   call the dispatcher.
3. The dirty count of 03 3.10: `Status` also counts the changed files
   from the `--porcelain=v2` records it already reads (ordinary,
   unmerged and untracked records; renames are off, so a rename counts
   as two). T059's facts and so salvage's `dirty-at-last-facts:<n>`
   (REQ-0237) read the count, not the number of kinds.

Open, for the maintainer before building: the sandbox kit installs its
own hook dispatcher as the system `core.hooksPath`, and the names gate
of T112 runs there. A global value wins over the system one, and the
chain of REQ-0408 (julieta's action, the tracked hook, the local hook)
has no stage for a system hooks path, so pointing the global key at
julieta's dispatcher would stop the kit's dispatcher, and the gate with
it. Options:

- (a) Recommended: `julieta setup` reads the system `core.hooksPath`
  (`git config --system`) and REQ-0408 gains a first `system` stage that
  runs that path's hook when one is set, so both dispatchers run and
  julieta stays the one global path. A change to REQ-0408 and REQ-0397
  on the `spec` surface, in this task.
- (b) julieta does not set the global key when a system one is set, and
  the kit's dispatcher calls `julieta hooks run`; a change in the
  sandbox kit repository, outside this repository, and T054's row 3
  stays open there.

- Implements: none: the verbs serve REQ-0688 (T060), REQ-0397 (T054)
  and the dirty count of 03 3.10 (T059, REQ-0237).
- Operator: no.
- Acceptance:
  - Each verb is tested first against real git, as a caller hits it:
    `ListRefs` over a repository with branches, tags, notes and two
    generations of `refs/salvage/`, returning the named namespaces only
    and refusing a namespace outside the allowlist; `Reflog` over a
    branch moved by commit, reset and amend; `BundleOf` with a pair
    whose commit is reachable from no ref (a reflog-only commit), with
    bases, and with no pair.
  - After `BundleOf`, the user's repository holds the same refs,
    `objects/info/alternates` and config as before (compared byte for
    byte, as I2's E case compares a clone), and no temporary repository
    remains, on success and on a forced failure.
  - `git bundle verify` passes on each bundle, which names exactly the
    pairs given, and `gitsafe.Unbundle` of it into an empty repository
    with the bases present restores them.
  - `SetGlobalHooksPath` sets the key in the sandbox's global config,
    leaves every other key as it was, and is refused in host mode; the
    argv and environment tests pin its command.
  - `julieta setup` points the global key at the dispatcher, and a
    commit in a test repository runs `julieta hooks run`; with the
    option the maintainer picks, a hook of a system `core.hooksPath`
    still runs.
  - `Status` counts files: two unstaged files and one untracked file
    read as 3; T059's facts golden shows the count.
  - REQ-0397, REQ-0408 and any other text the change touches say what
    the code does; `go run ./tools/ci reqs` passes.
- Verify: `go test -race ./internal/gitsafe/... ./internal/cmd/julieta/...`
  and `go run ./tools/ci reqs`.

T060 is re-planned on this task once it merges: its bundle comes from
`ListRefs`, `Reflog`, `Worktrees` and `BundleOf`, and its `needs` gains
T153.
