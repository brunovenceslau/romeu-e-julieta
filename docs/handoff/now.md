# Current state

## Where we stopped

Checkpoint of 2026-10-17. #67 and #69 merged, green on the four runners
(Done records 5 and 6): every row of the step table in 10 10.2 carries
a requirement id, and the pre-push hook and every mise run of
`tools/ci` install nothing by themselves (T138).

## Next steps

1. T012 waits by decision. T106, T107, T139 and T140 follow T105,
   which has merged; T108 follows T107. T106 is split in two pull
   requests: `tools/new` kinds by file with the `task` kind, then the
   `tools/ci plan` check. T107's operator items are decided
   ("re-plan-items-8-9-and-11-the-ask-first-surfaces").
2. Tests bound by wall-clock time ("tests-bound-by-wall-clock-time").
3. Migrate the docs/reviews checkpoints
   ("migrate-the-docs-reviews-checkpoints-into-docs-handoff").
4. The go that builds tools/ci
   ("the-go-that-builds-tools-ci-is-not-held-to-the-pin").

## Open for the operator

One block, a recommendation each.

1. I29's code point set ("i29-and-the-invisible-code-points-outside-it").
   Recommended: widen it to Default_Ignorable_Code_Point before M1's
   gate diff.
2. The kit prompts ("prompts-for-the-sandbox-kit-and-the-house-kit").
   Recommended: ask for both final prompts in one message.

## In flight

Nothing.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
