# Current state

## Where we stopped

Checkpoint of 2026-10-18, after the overnight run. Done records are
merged up to 33. GitHub stack #136 holds, bottom to top: the ADR 0016
stack (#132 to #135, Done record 34), then #137 to #146 (T009, T024,
T025, T124, T128, T131 and their re-plans) and this record PR (Done
record 35). None is merged.

## Next steps

1. Merge the stack bottom to top, or the part of it that is ready; the
   record PR then ticks each merged task (`fate: merged`, `merged:`,
   `pr:`), as #135 does for T008.
2. After the merge: T132 (pending note "t132-after-the-overnight-stack"),
   then T010 once the import question is answered.

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
   globs to `checks`, in a pull request of its own, together with
   `tools/ci/internal/crossbuild/**`, the package of T009's check, which
   no glob of `checks` matches either.
4. cobra's output outside the CLI writer in completion
   ("cobra-writes-outside-the-cli-writer-in-completion"). Recommended:
   fix the stderr path in `Main` and unset `BASH_COMP_DEBUG_FILE`.
5. T125's three design questions ("t125-three-design-questions").
   Recommended: a version file, the fixture schema in a test, and a
   start after T009 merges.
6. `tools/new` importing `internal/cli` against REQ-0971
   ("tools-new-imports-internal-cli"). Recommended: one sentence in
   REQ-0971 that allows it.
7. The lint set of T139 (item 16 of Decisions of the re-plan), decided
   at a sitting before T139 starts. Recommended: option A, the agreed
   default.
8. The operator steps the run skipped: the label `deferred` (T124),
   mise's public key out of band and the network policy (T012), the
   sittings of T112 (M1) and T126 (O8), and the product records of T123,
   which ADR 0016 moved to start at 0017.
9. NFC and NFD names in the tree walk
   ("nfc-and-nfd-names-in-the-tree-walk"). Recommended: add
   `golang.org/x/text` to REQ-1049.

## In flight

- GitHub stack #136: #132 to #135, #137 to #146 and the record PR on
  top.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
