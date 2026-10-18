# Current state

## Where we stopped

Checkpoint of 2026-10-18. Done records are merged up to 33. ADR 0016
(the generated header) is in review: #132 (the record and the spec),
#133 (re-plan), #134 (the code) and the record PR on top (Done record
34).

## Next steps

1. The next tasks of the plan, in order, stacked on the ADR 0016 stack
   (decision note "the-overnight-run-of-2026-10-18"). T132 and T123
   each need a plan fix first (pending note
   "t132-and-t123-owns-and-the-build-constraint-gap").

## Open for the operator

One block, a recommendation each.

1. I29's code point set ("i29-and-the-invisible-code-points-outside-it").
   Recommended: widen it to Default_Ignorable_Code_Point before M1's
   gate diff.
2. The kit prompts ("prompts-for-the-sandbox-kit-and-the-house-kit").
   Recommended: ask for both final prompts in one message; the re-run
   rule is now in that note.
3. The T140 tests and the checks surface
   ("the-t140-tests-and-the-checks-surface"). Recommended: add both
   globs to `checks`, in a pull request of its own.
4. cobra's output outside the CLI writer in completion
   ("cobra-writes-outside-the-cli-writer-in-completion"). Recommended:
   fix the stderr path in `Main` and unset `BASH_COMP_DEBUG_FILE`.

## In flight

- The ADR 0016 stack: #132, #133, #134 and its record PR.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
