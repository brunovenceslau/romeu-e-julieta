# Current state

## Where we stopped

Checkpoint of 2026-10-17. #67 to #71 merged (Done records 5 to 7):
T138, and T106's first half (`tools/new` kinds by file and the `task`
kind). T106's second half, the `plan` check, is open as #72 and marks
T106 merged. T107 has started (see In flight).

## Next steps

1. #72 merged and green; then the handoff PR that records it closes
   "check-that-merged-and-pr-come-as-a-pair".
2. T107 ("t107-one-source-for-the-ask-first-list-in-progress"), its
   decisions in "the-ask-first-surfaces-of-t107"; then T108.
3. T012 waits by decision. T139 and T140 follow T105, which has merged.
4. Tests bound by wall-clock time ("tests-bound-by-wall-clock-time").
5. Migrate the docs/reviews checkpoints
   ("migrate-the-docs-reviews-checkpoints-into-docs-handoff").
6. The go that builds tools/ci
   ("the-go-that-builds-tools-ci-is-not-held-to-the-pin").
7. Split the tasks over the Implements cap
   ("split-the-open-tasks-over-the-implements-cap").

## Open for the operator

One block, a recommendation each.

1. I29's code point set ("i29-and-the-invisible-code-points-outside-it").
   Recommended: widen it to Default_Ignorable_Code_Point before M1's
   gate diff.
2. The kit prompts ("prompts-for-the-sandbox-kit-and-the-house-kit").
   Recommended: ask for both final prompts in one message.
3. Merge #72, which carries the `checks` and `approvals` approval
   lines. Recommended: merge once its four runners are green.

## In flight

- #72, branch `feat/t106b-tools-ci-plan`: the `plan` check.
- Branch `feat/t107-one-source-for-the-ask-first-list`, local only: one
  draft commit of T107's first step; the pending note says what it
  holds.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
