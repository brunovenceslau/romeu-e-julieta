# Current state

## Where we stopped

Checkpoint of 2026-10-17. Done records are merged up to 30. The cobra
stack (#115, #116, #117) merged without the adjustment for generators
with their data, which a docs stack now carries (ADR 0015 accepted).
T008's first implementation (#109) is held. T130 is in stack #114
(#112, #113).

## Next steps

1. The operator merges the docs stack for generators with their data
   (ADR 0015 decisions 7 and 8, REQ-1049, REQ-1246, T008's `owns`).
2. Renumber #113's Done record (it holds 0031, written before 0030
   became the cobra record) with `handoff_check.py --fix --base
   origin/main`; then the operator merges stack #114.
3. Rename `tools/ci/internal/generated` to `drift` (pending note
   "rename-the-generated-package-to-drift").
4. Rework T008 on cobra in #109 (retarget it to main). The error table,
   the union test, Report and `--json`, the `errors.md` generator and
   RequireID carry over; the dispatcher, help and registry become
   cobra. A new record PR goes on top.
5. Then T009 and T025. T132 and T123 each need a plan fix first
   (pending note "t132-and-t123-owns-and-the-build-constraint-gap",
   which arrives with #113).

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

- The docs stack for generators with their data.
- Stack #114 (#112, #113).
- #109, held.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
