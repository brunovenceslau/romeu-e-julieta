# Current state

## Where we stopped

Checkpoint of 2026-10-17. #67 to #73 merged (Done records 5 to 8):
T138 and T106, both halves (`tools/new` kinds by file and the `task`
kind, then the `plan` check). T107 has started (see In flight).

## Next steps

1. T107 ("t107-one-source-for-the-ask-first-list-in-progress"), its
   decisions in "the-ask-first-surfaces-of-t107".
2. x64 becomes best effort
   ("x64-is-best-effort-on-a-scheduled-run"). Then T108.
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

## In flight

- Branch `feat/t107-one-source-for-the-ask-first-list`: one commit on
  `main` after #72 with all of T107's list; the pending note says what
  is left.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
