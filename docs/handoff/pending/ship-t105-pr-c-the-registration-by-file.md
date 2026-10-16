# Ship T105 PR-C, the registration by file

The branch feat/t105c-registration holds a builder's work from
2026-10-16, with the builder's adversarial review of its own diff.

What the builder measured:

- `tools/ci all` and `fast` are green.
- The step lists of fast, of fast run as the pre-push hook, and of all
  are equal before and after the move.
- There were 296 tests before and 327 after, and none was lost.
- REQ-1234 was already covered by TestMainSecondSignalAbsorbed.

To do, in order:

1. Rebase onto main with signing on. docs/spec/10-testing-style.md and
   the T141 task file changed on main on 2026-10-16.
2. Run the ship gate: code-reviewer and security-auditor. The diff
   touches the checks, approvals and ask-first surfaces.
3. Open the PR with these approval lines: checks, approvals, ask-first,
   and spec. The spec line is needed because 05 5.3's copy of the
   ask-first list follows the moved glob.

Points the PR states:

- The usage text now follows the registry order.
- mise.WhichPinned refuses a tool that is not in PinnedTools.
- The hygiene and workflows rows of 10 10.2 carry no REQ id, so
  checks.md shows "-" for them.
- These citations of moved paths are outside T105's owns and were not
  edited:
  - 05-security.md:361
  - docs/spec.md:362
  - a comment in ci.yml
  - a comment in .golangci.yml
  - ADR 0007
  - the T005 and T128 task files
- The PR that merges marks T105 merged, and T137 with it (pending item
  "mark-t137-merged-together-with-t105").

Reopen: the next session starts.
