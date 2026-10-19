---
id: T142
title: "`gitsafe`: the read-only plan history"
milestone: M1
module: gitsafe
fate: new
owns:
  - internal/gitsafe/history*.go
needs:
  - T024
after: []
ask_first:
  - gitsafe
sitting: none
---

# T142 - `gitsafe`: the read-only plan history

Goal: the two read-only verbs of `gitsafe` that the trends of
`tools/planviewer` read the plan's past through: `History`, the commits
that touched a path on the first-parent history of a revision, and
`ReadDir`, the regular files directly under a directory at a commit.

Re-plan (2026-10-18): New. A later view of `tools/planviewer` shows how
the plan changed at each commit that touched it, so it needs the task
files as they were at each past commit, and every git invocation goes
through `gitsafe`. The maintainer approved the change to the `gitsafe`
surface on 2026-10-18. The task adds the verbs and their tests in
`history*.go` and `runInput`, the standard input of the runner, in
`gitsafe.go`; the trend view itself is not part of it.

- Implements: REQ-1253 (12 12.2).
- Operator: no.
- Acceptance:
  - `History` lists the commits newest first, leaves out a commit of a
    merged branch, takes the path literally and gives no commit for a
    path no commit touched.
  - `ReadDir` skips a directory below the path and refuses a symbolic
    link, a gitlink, a name the tree walk refuses, a collision and a
    blob over 1 MiB, reading every blob with one `cat-file --batch`.
  - Each bound of REQ-1253 is tested with the bound lowered, and the
    readers of git's output give the same result in chunks of any size.
  - A promisor remote on a local listener is never reached, a graft
    file changes nothing and a shallow repository is refused, each with
    plain git as the positive control where one exists.
  - A `gpg.program` and `log.showSignature` planted in the local config
    never run, and plain git with the same config runs them.
  - git's standard error reaches the errors of both verbs escaped.
- Verify: `go test ./internal/gitsafe/... -run 'History|ReadDir'`.
