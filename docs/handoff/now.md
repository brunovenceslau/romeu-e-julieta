# Current state

## Where we stopped

Checkpoint of 2026-10-17. #67 to #94 merged (Done records 5 to 25):
T138, T106, T107, x64 best effort on a scheduled `x64.yml`, T108 (ADR
0014, which closes the decisions of the re-plan), the procgroup drain
fix (Done record 12), the docs/reviews checkpoints moved here (Done
record 22), tools/new refusing a flag as a title (Done record 23), and
T140, the module tree and the spec identifiers held to the code, with
its spec fix (Done record 24).

## Next steps

1. Take a task from `go run ./tools/ci plan --ready`. M1's foundation
   packages start beside M0 (docs/plan.md, "Milestones and sittings"):
   T007 (termsafe), T017 (digests), T023 (gitsafe, contracts). Each
   ask-first surface its task file forecasts needs the operator's
   approval first; no grant is written before sitting M0.
2. The other ready tasks: T123, T125, T128, T129, T130, T132; T124
   needs an operator API call; T112 runs at sitting M1, T126 at O8.
3. T012 waits by decision. T139 waits for item 16 (the lint set) at
   sitting M0.
4. At their triggers: "tight-wall-clock-bounds-left-in-the-tests",
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

- This handoff's pull request.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
