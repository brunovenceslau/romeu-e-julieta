# Current state

## Where we stopped

Checkpoint of 2026-10-20, after T127 and the re-plan of T051 merged
(Done record 43). Done records are merged up to 42.

## Next steps

1. T143, the `gitsafe` read verbs, then T051, which needs it; both
   under the standing grants of "the-grants-of-2026-10-19" (T143's
   `gitsafe` surface is not granted: its pull request carries an
   approval line).
2. T144, the follow-ups of the container e2e, whose task file lands with
   its own pull request; its `checks` surface is not in the M0 grant, so
   its pull request carries an approval line.
   Then the docs pull request of
   "the-spec-catches-up-with-the-hardening-of-t127-t143-and-t144".
3. The rest of the ready list (`go run ./tools/ci plan --ready`).
4. T112 lands through the sessions of the config and kit repositories
   ("t112-in-the-config-and-kit-repositories"); then this repository
   drops the hashed denylist and the CI check over pull request text.

## Open for the operator

One block, a recommendation each. Decisions of 2026-10-19 are in
"the-grants-of-2026-10-19".

1. Paste the two T112 prompts, the config repository's first.
   Recommended: today, so the worktree hook lands before the next
   fan-out of subagents.
2. The sitting of T126 (O8) waits for the GitHub App of
   [ADR 0011, keep the administrator's pull-request bypass and give the sandbox a GitHub App token](../adr/0011-keep-the-administrator-s-pull-request-bypass-and-give-the-sandbox-a-github-app-token.md),
   which does not exist yet. Recommended: create it when M3 starts.
3. T111 (sitting M1) waits for T051, T054, T059, T060, T078 and T035.

## In flight

- T143 (branch `feat/t143-gitsafe-status`) and T144 (branch
  `fix/t013-leftovers`), each built by a subagent in a worktree outside
  the repository.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
