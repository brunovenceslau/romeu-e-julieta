# Current state

## Where we stopped

Checkpoint of 2026-10-15, written before a sandbox recreate. T105 PR-A
merged (Done record 2). The other five lanes were mid-flight when every
subagent stopped on an authorization error (gotcha
"subagents-can-fail-with-an-organization-authorization-error"); each
lane was pushed to its branch, some as one WIP commit, none gated.

## Next steps

1. Probe one small subagent first; fan out only if it works.
2. T105 PR-B: pending item "finish-t105-pr-b-the-procgroup-switch".
3. T007 and T017: pending item "ship-t007-termsafe-and-t017-canon".
4. Spec fixes PR, then T023 stacked on it: pending items
   "ship-the-spec-fixes-pr" and "finish-t023-gitsafe".
5. Merge sitting when three PRs are green.
6. PR-C of T105 (the register-by-file move) after PR-B merges; it marks
   T105 and T137 merged ("mark-t137-merged-together-with-t105").
7. T012 waits by decision. T106, T107, T108 and T138 follow T105.
8. Migrate the docs/reviews checkpoints
   ("migrate-the-docs-reviews-checkpoints-into-docs-handoff").

## Open for the operator

One block, a recommendation each.

1. Merge sitting: when three PRs are green. Recommended: hold the
   sitting then, not before.
2. T012 waits for its own focus. Recommended: start it after T105 PR-B.
3. The subagent authorization error. Recommended: confirm Claude Code
   access for subagents before the next fan-out.

## In flight

Each lane's branch (measure state with
~/.sbx-kit/claude-home/bin/handoff_state.py; nothing here is current):
- T105 PR-B: feat/t105b-procgroup-switch (WIP commit)
- T007: feat/t007-termsafe
- T017: feat/t017-canon
- Spec fixes: docs/spec-fixes-2026-10-13
- T023: feat/t023-gitsafe, on the spec fixes branch (WIP commit)

Branch and pull request state is NOT recorded here: measure it.
