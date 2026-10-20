---
id: T143
title: "`gitsafe`: the read verbs of a working tree"
milestone: M1
module: gitsafe
fate: merged
owns:
  - internal/gitsafe/status*.go
  - internal/gitsafe/gitsafe.go
needs:
  - T023
after: []
ask_first:
  - gitsafe
sitting: none
merged: 2026-10-20
pr: 193
---

# T143 - `gitsafe`: the read verbs of a working tree

Merged on 2026-10-20 in pull request #193. The text below is the one
written at the re-plan.

Goal: the read-only verbs of `gitsafe` that `julieta setup` and `julieta
status` read a repository's working tree through: the checked-out
branch and HEAD, whether the working tree is clean, the local branch
heads, the default branch of `origin` (`refs/remotes/origin/HEAD`),
ahead and behind between two refs, the stashes, the worktrees, and the
repo-local `core.hooksPath`, and the object id a ref points at
(the tip of `refs/remotes/origin/<default>` before and after a
fast-forward, for `default-branch-moved`). T051 composes "checked out, clean and
fast-forwardable" from these reads and the existing `FastForward`.

Re-plan (2026-10-20): New. T051 found that `gitsafe` has no verb that
reads the state of a working tree, while 04 4.3 runs every julieta git
call in sandbox mode (REQ-0396), `setup` fast-forwards only a clean
default branch (REQ-0399) and `status` reports the per-repo fields of
REQ-0424. Reading `.git` directly would bypass `gitsafe`. The maintainer
chose on 2026-10-20 a task of its own, so the `gitsafe` change is
reviewed apart from the commands; T051 needs it. The task also adds
`status*.go` to the file families listed in the package comment of
`gitsafe.go`, as T142 did for `history*.go`.

- Implements: none: the verbs serve REQ-0396, REQ-0399 and REQ-0424,
  which T051 implements.
- Operator: no.
- Acceptance:
  - Each verb runs one git command through the runner, except Status,
    which runs four (the filter drivers of the config, `symbolic-ref`
    for HEAD, `status`, and `ls-files -v` for the assume-unchanged and
    skip-worktree flags, which make the tree not clean), in the mode
    the caller gives, with the flags and environment of that mode, and
    returns typed values, not git's text; a `dir` that is not a
    repository root is refused as the other verbs refuse it. A ref name
    in a result passes the display check (no character `unsafeRune`
    refuses), or the read fails; a stash message is escaped and cut,
    not refused.
  - `RefOID` returns the object id a full ref name points at, and a
    missing ref as missing, not as an error.
  - Every verb runs with `GIT_OPTIONAL_LOCKS=0` (10 10.2 does the same for `git status`),
    so no read refreshes the index or takes `index.lock`, and no verb
    runs a repository-configured filter or hook: "clean" is read without
    running a clean filter.
  - A clean tree, a tree with a staged change, one with an unstaged
    change and one with an untracked file are each read correctly, one
    fixture repository each; a detached HEAD and an unborn branch are
    read without error.
  - Ahead and behind are counted between two refs the caller names (a
    branch and its upstream, or a local default branch and
    `origin/<default>`), with a fixture on each side and one diverged; a
    branch with no upstream says so instead of reporting zero.
  - Fixtures cover several local branches, one with an unusual but legal
    name; zero, one and many stashes; the main worktree, a linked one and
    a prunable one.
  - The repo-local `core.hooksPath` is read with `git config --local`
    only: a `core.hooksPath` planted in the global config and the
    `/dev/null` that sandbox mode forces are each not reported.
  - Each list (branches, stashes, worktrees) and git's standard error
    are capped, and a test lowers the cap to show the bound, as T142's
    verbs do.
  - No verb writes to the repository: each test checks that the refs,
    the config, and the index's bytes and modification time are
    unchanged after the call.
- Verify: `go test ./internal/gitsafe/... -run '^TestStatus' -v`.
