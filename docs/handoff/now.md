# Current state

## Where we stopped

Checkpoint of 2026-10-16, before a /clear. Eight pull requests merged
(#53, #55 to #61; Done record 3). The five lanes saved on 2026-10-15
all shipped. T105 PR-C is built and pushed but not yet gated.
Subagents worked this session.

## Next steps

1. Ship T105 PR-C: pending item "ship-t105-pr-c-the-registration-by-file".
2. T141, stacked on PR-C: pending item
   "t141-setup-installs-the-pinned-mise".
3. Spec PR for the step-table rows that have no REQ id
   ("step-table-rows-without-a-req-id").
4. T012 waits by decision. T106, T107, T108, T138, T139 and T140
   follow T105.
5. Migrate the docs/reviews checkpoints
   ("migrate-the-docs-reviews-checkpoints-into-docs-handoff").

## Open for the operator

One block, a recommendation each.

1. I29's code point set ("i29-and-the-invisible-code-points-outside-it").
   Recommended: widen it to Default_Ignorable_Code_Point before M1's
   gate diff.
2. The kit prompts ("prompts-for-the-sandbox-kit-and-the-house-kit").
   Recommended: ask for both final prompts in one message.
3. The merge sitting for PR-C and T141. Recommended: merge PR-C first.
   T141 rides on it.

## In flight

- T105 PR-C: branch feat/t105c-registration, no PR yet.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
