# Current state

## Where we stopped

Checkpoint of 2026-10-17. Done records are merged up to 29. T008's first
implementation (#109) is held while the command line moves to cobra (ADR
0015, Done record 30, stack #118). T130 is in stack #114 (#112, #113).

## Next steps

1. The operator merges stack #118 after the adjustment of the pending
   note "adjust-the-cobra-stack-for-generators-with-their-data". Do the
   adjustment first, on #115 and #116, then run the gate again.
2. Renumber #113's Done record (it holds 0031, written before 0030
   became the cobra record) with `handoff_check.py --fix --base
   origin/main` after #118 merges; then the operator merges stack #114.
3. Rework T008 on cobra in #109 (retarget it to main). The error table,
   the union test, Report and `--json`, the `errors.md` generator and
   RequireID carry over; the dispatcher, help and registry become
   cobra. A new record PR goes on top.
4. Then T009 and T025. T132 and T123 each need a plan fix first
   (pending note "t132-and-t123-owns-and-the-build-constraint-gap",
   which arrives with #113).

## Open for the operator

One block, a recommendation each.

1. The `*_gen.go` header question and the rename of the package
   `tools/ci/internal/generated`
   ("adjust-the-cobra-stack-for-generators-with-their-data").
   Recommended: no generated-code header; rename it to `drift`.
2. I29's code point set ("i29-and-the-invisible-code-points-outside-it").
   Recommended: widen it to Default_Ignorable_Code_Point before M1's
   gate diff.
3. The kit prompts ("prompts-for-the-sandbox-kit-and-the-house-kit").
   Recommended: ask for both final prompts in one message; the re-run
   rule is now in that note.
4. The T140 tests and the checks surface
   ("the-t140-tests-and-the-checks-surface"). Recommended: add both
   globs to `checks`, in a pull request of its own.
5. Accept ADR 0015 (Proposed) when merging #115.

## In flight

- Stack #118 (#115, #116, #117).
- Stack #114 (#112, #113).
- #109, held.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
