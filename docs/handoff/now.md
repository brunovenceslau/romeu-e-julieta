# Current state

## Where we stopped

Checkpoint of 2026-10-17. #67 merged, green on the four runners (Done
record 5): every row of the step table in 10 10.2 carries a
requirement id.

## Next steps

1. T012 waits by decision. T106, T107, T108, T138, T139 and T140
   follow T105, which has merged.
2. Tests bound by wall-clock time ("tests-bound-by-wall-clock-time").
3. Migrate the docs/reviews checkpoints
   ("migrate-the-docs-reviews-checkpoints-into-docs-handoff").

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
