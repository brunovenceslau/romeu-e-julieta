# Current state

## Where we stopped

Checkpoint of 2026-10-13, written at a context stop. Done record 1 holds
what merged this session. Five lanes were mid-build when the context
ended: T105 PR-A, T105 PR-B, T007, T017 and T023. The next session
measures their state (see In flight).

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
2. T012 waits for its own focus. Recommended: start it after T105
   PR-B (it touches pins and a signing key and deserves its own focus).

## In flight

Map of lanes (names only), each in a worktree in the sandbox scratch:
- T105 PR-A: docs/t105a-tool-timeouts
- T105 PR-B: feat/t105b-procgroup-switch
- T007: feat/t007-termsafe (PR with a termsafe approval line)
- T017: feat/t017-canon (PR with a digests approval line)
- T023: feat/t023-gitsafe (PR with a gitsafe approval line)

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
