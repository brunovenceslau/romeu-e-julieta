# Remove gitsafe FastForward, which no command calls since T177

Since T177 (2026-10-28), `julieta setup` moves no branch, and
`gitsafe.FastForward` (`internal/gitsafe/fetch.go`) has no caller
outside its own tests; 05 5.1 still describes it. T177 left it in
place, since a change to `gitsafe` is ask-first and belongs to a gitsafe
task. If it stays, the cost is unused code that keeps a verb able to
move a branch, and its tests to maintain.

Remove the verb, its tests and its lines in 05 5.1, with the `gitsafe`
and `spec` approvals.

Scope: this repository, `internal/gitsafe` and 05 5.1.

Reopen: the next re-plan, or the next task that changes `gitsafe`.
