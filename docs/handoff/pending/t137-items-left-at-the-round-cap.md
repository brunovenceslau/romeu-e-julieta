# T137: items left at the round cap

Ship gate round 3 GO at 50da992 (internal/procgroup).

- L6: a Stdout writer that blocks holds Run's return past the limit (a
  300ms limit returned after 6.0s; the group dies at the limit). Reword
  Command.Stdout's doc, or make drain stop waiting at the bound.
- L7: CombinedOutput has no memory cap; T056 and the hooks audit confirm
  that no Unbounded-class caller uses it.
- Info, for T039's scan: Syscall9 is missing (it exists on darwin); the
  unix.ForkExec entry is inert; TIOCSTI passes through the SYS_IOCTL
  allowance; .s files are not scanned; spawns through transitive packages
  are missed (a `go list -deps` reachability check); a literal Kill(-1)
  or FindProcess(-1).Kill() is not flagged; a Stdin reader that may block
  must not be reused after Run (lost input).
- Tests: leaderEnded is untested (pin it with an fd count while the stdin
  reader is blocked); a regular file passed as Stdout or Stdin is
  unpinned; add comments on the equivalent survivors (cancel-leader-only,
  drain-close-loop-removed); macOS is not covered by these cases.
- Nit: procgroup.go lines 23-24 line width.

Reopen: T039 starts (the scan items); the next change to
internal/procgroup (the rest).
