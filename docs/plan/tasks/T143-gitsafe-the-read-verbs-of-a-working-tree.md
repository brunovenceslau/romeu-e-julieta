---
id: T143
title: "`gitsafe`: the read verbs of a working tree"
milestone: M1
module: gitsafe
fate: new
owns:
  - internal/gitsafe/status*.go
needs:
  - T023
after: []
ask_first:
  - gitsafe
sitting: none
---

# T143 - `gitsafe`: the read verbs of a working tree

Goal: the read-only verbs of `gitsafe` that `julieta setup` and `julieta
status` read a repository's working tree through: the checked-out
branch and HEAD, whether the working tree is clean, the local branch
heads, ahead and behind against the upstream, the stashes, the
worktrees, and the repo-local `core.hooksPath`.

Re-plan (2026-10-20): New. T051 found that `gitsafe` has no verb that
reads the state of a working tree, while 04 4.3 runs every julieta git
call in sandbox mode (REQ-0396), `setup` fast-forwards only a clean
default branch (REQ-0399) and `status` reports the per-repo fields of
REQ-0424. Reading `.git` directly would bypass `gitsafe`. The maintainer
chose on 2026-10-20 a task of its own, so the `gitsafe` change is
reviewed apart from the commands; T051 needs it.

- Implements: none: the verbs serve REQ-0396, REQ-0399 and REQ-0424,
  which T051 implements.
- Operator: no.
- Acceptance:
  - Each verb runs one git command through the runner, in the mode the
    caller gives, with the flags and environment of that mode, and
    returns typed values, not git's text.
  - A clean tree, a tree with a staged change, one with an unstaged
    change and one with an untracked file are each read correctly, one
    fixture repository each; a detached HEAD and an unborn branch are
    read without error.
  - Ahead and behind are counted against the branch's upstream; a branch
    with no upstream says so instead of reporting zero.
  - The repo-local `core.hooksPath` is read from the repository's own
    config only, never from the global or system config.
  - No verb writes to the repository: each test checks that the refs,
    the index and the config are unchanged after the call.
- Verify: `go test ./internal/gitsafe/... -run '^TestStatus' -v`.
