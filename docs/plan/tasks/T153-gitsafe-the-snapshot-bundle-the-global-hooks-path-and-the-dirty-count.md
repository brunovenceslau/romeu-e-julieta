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
  - internal/hooks/hooks.go
  - internal/hooks/hooks_test.go
  - internal/hooks/system*.go
  - internal/cmd/julieta/hooks/run.go
  - internal/cmd/julieta/hooks/run_test.go
  - internal/cmd/julieta/setup/hooks_path*.go
  - internal/cmd/julieta/handoff/facts*.go
  - internal/handoff/handoff.go
  - e2e/julieta_hooks_test.go
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

Goal: the `gitsafe` verbs that two merged tasks (T054, T059) and one
open task (T060) wait for, in one task on the `gitsafe` surface, as
decided on 2026-10-20 (one task for the two pending notes below,
approved for planning).

1. The snapshot bundle of REQ-0688, which T060 (`julieta snapshot`)
   cannot build today ("gitsafe verbs for the snapshot bundle",
   01m5d7v42kv79hf42g15ffgk59). `Branches` lists only `refs/heads`. No
   verb reads a reflog. `BundleCreate` takes only refs that exist.
   `CreateRef` is create-only and limited to `refs/salvage/` in sandbox
   mode. git refuses a bundle of a raw object id, a reflog entry or
   `--stdin` revisions alike ("Refusing to create empty bundle", git
   2.53.0, measured on 2026-10-20). This task adds three verbs:
   - `ListRefs`: the refs under the namespaces the caller names, from
     a closed list (`refs/heads/`, `refs/tags/`, `refs/notes/`, and
     `refs/salvage/<generation>/` for one generation per call), checked
     as plain prefixes, with their object ids, sorted by name and
     bounded as `Branches` is (`maxBranches`).
   - `Reflog`: the object ids of one ref's reflog entries, oldest
     first, bounded; a ref with no reflog is an empty list, and an entry
     whose commit is gone is left out.
   - `BundleOf`: a bundle of name and object id pairs, minus the bases.
     Each name passes `checkRefName` and is under `refs/heads/`,
     `refs/tags/`, `refs/notes/`, `refs/salvage/` or
     `refs/salvage-reflog/` (worktree HEADs are named by the caller under
     one of them, as T060 decides). No ref is written into the user's
     repository: a temporary bare repository under the caller's
     directory, of the user's object format, borrows the user's objects
     (the common directory's `objects` for a linked worktree), gets the
     refs and runs `git bundle create`, and is removed on every path,
     a cancelled context included. How it borrows the objects is
     decided while building and recorded with its reason:
     `objects/info/alternates`, or `GIT_ALTERNATE_OBJECT_DIRECTORIES` on
     those calls only (sandbox mode passes no other `GIT_*` variable).
     No pair is an error of its own, which the caller tells apart from
     git's.
2. The global `core.hooksPath` of REQ-0397, so git calls the dispatcher
   T054 installs ("gitsafe verbs for the global hooks path and the dirty
   count", 01m5d7v43hd4hsw60e4bhek3pw), and the system hooks path the
   sandbox kit sets keeps running (option (a), picked on 2026-10-21):
   - `SetGlobalHooksPath`, sandbox mode only, writes the key with `git
     config --global`, runs in no repository, and takes an absolute path
     that `unsafeRune` accepts. It first reads the global key and returns
     the value it found: it leaves the key alone when it already holds
     that path (REQ-0396), and writes nothing when it holds another
     value. Host mode refuses it (I2). julieta's
     own git calls keep `-c core.hooksPath=/dev/null`, so the global key
     does not make them call the dispatcher.
   - `SystemHooksPath` reads the system config's `core.hooksPath`
     (`git config --system`), with the checks of `HooksPath`.
   - `julieta setup` records that value in julieta state (REQ-0154)
     before it sets the global key, and the dispatcher reads the record,
     since it starts no git (T054, "Decisions taken while building").
     REQ-0408's chain gains a first `system` stage: the hook of the
     recorded path, when one is set and the hook is executable, with
     the hook's arguments and the same stdin rule; the failure line
     names it as stage `system`, and the help of `julieta hooks run`
     lists it. A recorded path that is not absolute is ignored, and one
     equal to julieta's own dispatcher path is not run, so the chain
     never calls itself. A global value `SetGlobalHooksPath` returns that
     is neither julieta's path nor empty is reported by `setup`, not
     overwritten.
   T054's `installDispatcher` calls both verbs, which closes its
   acceptance row 3, and the e2e case drops the line that set the key
   in its place (`e2e/julieta_hooks_test.go`).
3. The dirty count of 03 3.10: `Status` gains `Changed`, the number of
   distinct paths among its ordinary, unmerged and untracked records
   and its hidden index entries; a path that is both staged and unstaged
   counts once, and renames are off, so a rename counts as two. T059's
   `dirty` and so salvage's `dirty-at-last-facts:<n>` (REQ-0237) read
   `Changed`, not the number of kinds, and the `RepoFacts.Dirty`
   comment says so. 03 3.10, owned by the open T021, gets one sentence
   that defines the count; T153 amends it without owning the file, as
   T151 did with 05 5.1, and does not wait for T021.

REQ-0463 and 05 5.1 say the rest of the sandbox's git config is its
own; a verb that writes the global config fits that, and if the text
needs a word it is amended the same way (05 is owned by T039).

- Implements: none: the verbs serve REQ-0688, REQ-0397 and REQ-0237,
  which T060, T054 and T059 implement.
- Operator: no.
- Acceptance:
  - Each verb is tested first against real git, as a caller hits it:
    `ListRefs` over a repository with branches, tags, notes and two
    generations of `refs/salvage/`, returning the named namespaces only
    and refusing one outside the list; `Reflog` over a branch moved by
    commit, reset and amend; `BundleOf` with a pair whose commit is
    reachable from no ref, with bases, with a SHA-256 repository, from
    a linked worktree, and with no pair.
  - After `BundleOf`, the user's repository holds the same refs,
    `objects/info/alternates` and config as before (compared byte for
    byte), and no temporary repository remains, on success, on a forced
    failure and on a cancelled context.
  - `git bundle verify` passes on each bundle, which names exactly the
    pairs given, and `gitsafe.Unbundle` of it into an empty repository
    with the bases present restores them.
  - `SetGlobalHooksPath` sets the key, leaves every other key of the
    global config as it was, is a no-op on a second run, and is refused
    in host mode; with another global value planted, it writes nothing,
    returns that value, and `julieta setup` reports it and leaves it as
    it was; `SystemHooksPath` reads a planted system value; the
    argv and environment tests pin both commands, with
    `-c core.hooksPath=/dev/null` present.
  - In the container e2e, after `julieta setup` and with no line that
    sets the key by hand, a commit runs `julieta hooks run`, and with a
    system `core.hooksPath` planted, its hook runs first, as stage
    `system`, and a failure there stops the chain and names the stage;
    a recorded path that is relative, or equal to julieta's own, runs
    nothing.
  - `Status.Changed` counts paths: two unstaged files, one of them also
    staged, one untracked file and one assume-unchanged entry read as
    4 (the hidden entry's path read from `git ls-files -v`); a new test
    of `internal/cmd/julieta/handoff` pins the count in
    the facts it writes.
  - REQ-0397, REQ-0408 and the 03 3.10 sentence say what the code does;
    `go run ./tools/ci reqs` passes.
- Verify: `go test -race ./internal/gitsafe/... ./internal/hooks/...
  ./internal/cmd/julieta/...`, `go test -tags e2e ./e2e/... -run
  '^TestJulietaHooks' -v` (the reviewer reads the cases in the `-v`
  output, 10 10.8), and `go run ./tools/ci reqs`.

T060 is re-planned on this task once it merges: its bundle comes from
`ListRefs`, `Reflog`, `Worktrees` and `BundleOf`, and its `needs` gains
T153.

Decisions taken while building (2026-10-21), each with what reopens
it:

- `BundleOf`'s temporary bare repository borrows the user's objects
  through its own `objects/info/alternates` file, not
  `GIT_ALTERNATE_OBJECT_DIRECTORIES`: the file goes with the temporary
  repository, every git call keeps the environment of its mode (sandbox
  mode passes no other `GIT_*` variable), and a file line needs no list
  separator where the variable splits paths at a colon. The temporary
  repository is made in the directory of the bundle file, and has no
  shallow file, so a shallow repository's history is git's error.
  Reopened by a caller that bundles from a shallow repository.
- `SetGlobalHooksPath` and `SystemHooksPath` read their key with
  `--includes`, as git reads it when it runs a hook, so a value in an
  included file is seen and kept. `SystemHooksPath` is sandbox mode
  only, since host mode reads no system config. Reopened by a host
  caller.
- A global `core.hooksPath` that names another directory leaves the
  sandbox unusable (`setup` exits 1 and names it), since git then runs
  no dispatcher, and so does a system value `setup` cannot read, which
  also leaves the global key unset. Reopened by a warning slug for it
  in 04 4.4.
- The record of the system hooks path is
  `$HOME/.local/state/julieta/system-hooks-path.json`
  (`internal/hooks/system.go`); a system config that sets no value
  removes it, and a record the dispatcher cannot read fails the hook.
  The container e2e runs as `nobody`, who cannot write
  `/etc/gitconfig`, so its system-stage cases plant that record after
  `setup`; the read of a planted system config is
  `TestSystemHooksPathRealGit` of `internal/gitsafe`, through
  `GIT_CONFIG_SYSTEM`, which only the test adds. Reopened by a harness
  mount that can plant `/etc/gitconfig`.
- `Reflog` takes `HEAD` as well as a full ref name, so a worktree's
  HEAD reflog can be read from that worktree. Reopened by T060.
