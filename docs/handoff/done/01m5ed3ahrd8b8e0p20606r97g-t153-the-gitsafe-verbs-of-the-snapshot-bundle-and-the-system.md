# T153, the gitsafe verbs of the snapshot bundle and the system hooks stage

Merged on 2026-10-21 in #224, with the gate it passed: a code review and
a security audit, then a re-review and a re-audit of the fix rounds;
`go run ./tools/ci all` after the last commit; the container e2e
`TestJulietaHooks` and `TestJulietaHooksSystemStage`; CI green on both
hosts.

- `gitsafe` gains `ListRefs`, `Reflog`, `BundleOf`,
  `SetGlobalHooksPath` and `SystemHooksPath`, and `Status.Changed`.
- `julieta setup` points the global `core.hooksPath` at the dispatcher
  and records the system one, which the dispatcher runs as its first
  `system` stage.
- Decided on 2026-10-21: the `system` stage, so the sandbox kit's own
  dispatcher keeps running once the global key points at julieta; every
  stage gets its own copy of the hook's stdin, through a temporary file
  with no size limit; `Changed` counts assume-unchanged entries and not
  skip-worktree ones, so a clean sparse checkout reads 0. T153's task
  file holds every decision taken while building, each with what
  reopens it.

This record ticks T153. T060 is re-planned on it in the pull request
that carries this record.

Closed by this record:

- "gitsafe verbs for the snapshot bundle" (01m5d7v42kv79hf42g15ffgk59),
  by T153's `ListRefs`, `Reflog` and `BundleOf`.
- "gitsafe verbs for the global hooks path and the dirty count"
  (01m5d7v43hd4hsw60e4bhek3pw), by T153's `SetGlobalHooksPath` and
  `Status.Changed`; T054's acceptance row 3 is met.

Lessons:

- A security audit found what the plan's reviews did not: a stage that
  takes the hook's stdin starves the next one. Reading what a change
  shadows (a config key, a shared input) belongs to the plan.
- A bound that protects memory can turn into a git failure when the
  hook is one git aborts on (`reference-transaction`); a bound on a
  hook's input is a behaviour change, not a safety margin.
