# Current state

## Where we stopped

Checkpoint of 2026-10-18. Done records are merged up to 31. ADR 0015
is accepted, with the generators next to their data (#119, #120). T130
is merged (Done record 31). T008's first implementation (#109) is held
until the rename to `drift` lands.

## Next steps

1. Rename `tools/ci/internal/generated` to `drift` (pending note
   "rename-the-generated-package-to-drift"), in a pull request of its
   own on the `checks` surface.
2. Rework T008 on cobra in #109 (retarget it to main). The error table,
   the union test, Report and `--json`, the `errors.md` generator and
   RequireID carry over; the dispatcher, help and registry become
   cobra. A new record PR goes on top.
3. Then T009 and T025. T132 and T123 each need a plan fix first
   (pending note "t132-and-t123-owns-and-the-build-constraint-gap").

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

## In flight

- The stack of T130 (Done record 31).
- #109, held.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
