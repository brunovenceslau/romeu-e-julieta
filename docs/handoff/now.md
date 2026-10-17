# Current state

## Where we stopped

Checkpoint of 2026-10-17. #67 to #84 merged (Done records 5 to 12):
T138, T106, T107, x64 best effort on a scheduled `x64.yml`, and T108,
the re-plan record (ADR 0014), which leaves no item of Decisions of the
re-plan open. The wall-clock stack (Done record 12) fixes procgroup's
drain, which could cut a successful command's output. The session
checkpoints of docs/reviews are Done records 13 to 21 (Done record 22).

## Next steps

1. T012 waits by decision. T139 and T140 follow T105, which has merged.
2. The tight wall-clock bounds left
   ("tight-wall-clock-bounds-left-in-the-tests"), when one fails.
3. The go that builds tools/ci
   ("the-go-that-builds-tools-ci-is-not-held-to-the-pin").
4. Split the tasks over the Implements cap
   ("split-the-open-tasks-over-the-implements-cap").
5. tools/new takes a flag as a title
   ("tools-new-takes-a-flag-as-a-title").

## Open for the operator

One block, a recommendation each.

1. I29's code point set ("i29-and-the-invisible-code-points-outside-it").
   Recommended: widen it to Default_Ignorable_Code_Point before M1's
   gate diff.
2. The kit prompts ("prompts-for-the-sandbox-kit-and-the-house-kit").
   Recommended: ask for both final prompts in one message.

## In flight

- Nothing besides this handoff.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
