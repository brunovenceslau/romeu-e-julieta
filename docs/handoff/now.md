# Current state

## Where we stopped

Checkpoint of 2026-10-13, written at a context stop. Done record 1 holds
what merged this session. Five lanes were mid-build when the context
ended; their output is uncommitted in worktrees under
/tmp/claude-1000/romeu-wt/ (sandbox scratch, gone with the sandbox):
T105 PR-A (committed and signed, not pushed), T105 PR-B, T007, T017
and T023 (builders that stop before committing).

## Next steps

Measure first (see In flight), then continue per lane:

1. T105 PR-A (docs/t105a-tool-timeouts): single docs task, so ship gate
   only; push; PR; bottom of the stack under PR-B.
2. REQ-0300 exception list (pending item "req-0300-s-exception-list-must-name-the-tools-mutants-go"):
   a docs PR before or together with PR-B.
3. T105 PR-B (feat/t105b-procgroup-switch): commit signed, names gates,
   ship gate (code-reviewer opus, security-auditor opus, test-engineer
   sonnet), stack on PR-A. Add the T105 acceptance lines named in
   pending item "t137-signal-spec-pr-items-left-at-the-round-cap". PR-C (the
   register-by-file move) starts after B merges and marks T105 and
   T137 merged (pending item "mark-t137-merged-together-with-t105").
4. T007 (feat/t007-termsafe), T017 (feat/t017-canon), T023
   (feat/t023-gitsafe): commit, ship gate, push, PR with an unticked
   approval line (termsafe, digests, gitsafe), watch CI.
5. T012 waits by decision. T106, T107, T108 and T138 follow T105.
6. Migrate docs/reviews checkpoints into this tree (pending item
   "migrate-the-docs-reviews-checkpoints-into-docs-handoff").

## Open for the operator

One block, a recommendation each.

1. Merge sitting: when three PRs are green (docs/plan.md, decision 12 of
   the re-plan block). Recommended: hold the sitting then, not before.
2. T012 waits for its own focus. Recommended: start it after T105 PR-B.
3. Items 8 to 11 of the re-plan decision block (narrowing the checks
   and contracts surfaces, the grants, the surfaces). Recommended:
   decide them at T107 or sitting M0, whichever comes first.

## In flight

Branches (a map, not state): docs/t105a-tool-timeouts (pushed),
feat/t105b-procgroup-switch, feat/t007-termsafe, feat/t017-canon,
feat/t023-gitsafe; the last four are uncommitted builder output in
worktrees t105b, t007, t017 and t023.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
