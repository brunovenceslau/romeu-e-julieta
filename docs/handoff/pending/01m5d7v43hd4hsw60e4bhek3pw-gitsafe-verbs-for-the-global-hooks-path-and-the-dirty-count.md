# gitsafe verbs for the global hooks path and the dirty count

What is at stake: medium; until the first verb lands, `julieta setup`
writes the dispatcher's scripts but git does not call them, so no hook
of julieta runs; until the second, a handoff's `dirty` counts kinds of
change, not files, and its gap line misses edits within one kind.

Scope: this repository.

Decided on 2026-10-20, in the T054 and T059 pull requests: a `gitsafe`
task (ask first) after T151 adds a verb that sets the sandbox's global
`core.hooksPath` (T054's `installDispatcher` then calls it, closing its
acceptance row 3), and a count of changed files from the
`--porcelain=v2` records `Status` already reads (T059's facts and
salvage's `dirty-at-last-facts`, REQ-0237, then read it). julieta's own
git calls already empty `core.hooksPath`, so the global key does not
make them call the dispatcher.

Planned on 2026-10-20 as T153, one task with "gitsafe verbs for the
snapshot bundle" (01m5d7v42kv79hf42g15ffgk59). Decided on 2026-10-21:
the dispatcher gains a first `system` stage for the system hooks path
the sandbox kit sets (option (a) of T153).

Reopen: when T153 merges, and this note closes.
