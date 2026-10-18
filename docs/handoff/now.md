# Current state

## Where we stopped

Checkpoint of 2026-10-17. #67 to #94 merged (Done records 5 to 25):
T138, T106, T107, x64 best effort on a scheduled `x64.yml`, T108 (ADR
0014, which closes the decisions of the re-plan), the procgroup drain
fix (Done record 12), the docs/reviews checkpoints moved here (Done
record 22), tools/new refusing a flag as a title (Done record 23), and
T140, the module tree and the spec identifiers held to the code, with
its spec fix (Done record 24). #97 recorded T007, T017 and T023 as
merged (Done record 26), the stack #100 fixed the chatty holder
test of internal/procgroup (Done record 27), and the stack #103
merged T129 (Done record 28).

## Next steps

1. The stack of ADR 0015 (cobra) merges first; then T008 is reworked on
   cobra in #109 (Done record 30). T009 and T025 wait for T008.
   T130's stack #114 is ready; T132 and T123 each need a plan fix first
   (pending note "t132-and-t123-owns-and-the-build-constraint-gap",
   once #113 merges). Each ask-first surface a task file forecasts
   needs the operator's approval first.
2. T012 waits by decision. T139 waits for item 16 (the lint set) at
   sitting M0.
3. At their triggers: "tight-wall-clock-bounds-left-in-the-tests",
   "the-go-that-builds-tools-ci-is-not-held-to-the-pin",
   "split-the-open-tasks-over-the-implements-cap".

## Open for the operator

One block, a recommendation each.

1. I29's code point set ("i29-and-the-invisible-code-points-outside-it").
   Recommended: widen it to Default_Ignorable_Code_Point before M1's
   gate diff.
2. The kit prompts ("prompts-for-the-sandbox-kit-and-the-house-kit").
   Recommended: ask for both final prompts in one message.
3. The T140 tests and the checks surface
   ("the-t140-tests-and-the-checks-surface"). Recommended: add both
   globs to `checks`, in a pull request of its own.

## In flight

- The stack of ADR 0015, cobra for the command line (Done record 30).

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
