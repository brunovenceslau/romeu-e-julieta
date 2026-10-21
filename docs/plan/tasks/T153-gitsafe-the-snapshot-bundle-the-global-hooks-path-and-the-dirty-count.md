---
id: T153
title: "`gitsafe`: the snapshot bundle, the global hooks path and the dirty count"
milestone: M1
module: gitsafe
fate: merged
owns:
  - internal/gitsafe/refs_list*.go
  - internal/gitsafe/reflog*.go
  - internal/gitsafe/bundle*.go
  - internal/gitsafe/global*.go
  - internal/gitsafe/status*.go
  - internal/gitsafe/gitsafe_test.go
  - internal/gitsafe/helper_test.go
  - internal/gitsafe/gitsafe.go
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
  - docs/spec/02-layouts.md
needs:
  - T054
  - T059
  - T151
after: []
ask_first:
  - gitsafe
  - spec
sitting: none
merged: 2026-10-21
pr: 224
---

# T153 - `gitsafe`: the snapshot bundle, the global hooks path and the dirty count

Merged on 2026-10-21 in pull request #224. The text below is the one
written when it was built.

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
  `--includes --type=path`, as git reads it when it runs a hook, so a
  value in an included file is seen and kept and a leading `~` is
  expanded. `setup` reports a system value that is still not absolute,
  rather than record one the dispatcher would ignore, and compares the
  global value with the dispatcher dir once cleaned, so a trailing
  slash is not foreign. `SystemHooksPath` is sandbox mode
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
- Every hook stage gets its own copy of the hook's stdin (decided on
  2026-10-21; REQ-0408 says so), so a system hook that reads none cannot
  hide the ref lines from the tracked `pre-push`. The copy has no size
  bound, since a non-zero `reference-transaction` in its prepared state
  aborts the transaction, and a bound would fail a fetch or push of many
  refs that streamed before. The dispatcher copies the stdin to a 0600
  file of the temporary directory (`os.TempDir`, never the repository),
  opens it once for each of the three hook stages, and removes the path
  before any stage runs: each stage gets its own `*os.File` at offset 0,
  which procgroup hands to the child as it is, and no file remains once
  the stages start, whatever ends the process; only a process killed
  during the copy itself leaves one. Two exceptions go, as they are, to
  the first hook that runs: `proc-receive`, whose stdin is a protocol
  that git keeps open, and a stdin that is a character device, such as
  the terminal git-p4 gives its `p4-*` hooks, which never ends
  (`/dev/null` is one too, and reads as empty either way). Reopened by a
  temporary directory too small for a hook's stdin.
- `Status.Changed` counts assume-unchanged entries (lowercase tags of
  `git ls-files -v`) and not skip-worktree ones, which still set
  `Hidden`, so a clean sparse checkout reads 0 (decided on 2026-10-21;
  03 3.10 says so). It saturates at 100,000 paths: a working tree that
  changes more reads 100,000 with `ChangedTruncated` set, and no error,
  so a handoff or snapshot of a huge change still reads, and the set of
  counted paths stays bounded; the handoff `dirty` reads the cap.
  Reopened by a reader that needs the exact count past it.
- The system stage runs with `JULIETA_HOOKS_SYSTEM=1`, and a dispatcher
  started with it set skips the system stage, so a system hook that
  leads back to the dispatcher, a copy of its script included, ends
  after one more chain. Reopened by a system hook that must run nested.
- The system stage runs before julieta's action, so a failing system
  hook stops the chain before julieta's snapshot, by REQ-0409's rule
  that the first non-zero status stops. Reopened by a snapshot that
  must run whatever the system hook says.
- `BundleOf`'s temporary repository is made with
  `--ref-format=reftable` (git 2.45.0, inside the 2.45.4 floor;
  measured with git 2.53.0 here and 2.47.3 in the container image), so
  no ref name becomes a file name. `reflog exists` takes no
  `--end-of-options`: git 2.53.0 and 2.47.3 accept it there, 2.45.4 was
  not measured, and the ref is `HEAD` or starts with `refs/`. Reopened
  by a measurement at the floor.
- T153 owns `internal/gitsafe/gitsafe.go`, whose package doc lists the
  verb files, and `docs/spec/02-layouts.md`, whose REQ-0154 lists the
  recorded system hooks path; no open task owns either.

Deferred to T060, each with what reopens it:

- `ListRefs` fails whole on one ref name `checkDisplayRef` refuses.
  Reopened by T060, which decides whether a snapshot skips that ref
  and records it.
- A `.gitsafe-bundle-*` directory a killed process left behind is not
  cleaned up; callers ignore it. Reopened by T060, when it owns the
  snapshot directory.
