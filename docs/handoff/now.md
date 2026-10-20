# Current state

## Where we stopped

Checkpoint of 2026-10-20, after T051 merged (Done record 45). Done
records are merged up to 44.

## Next steps

1. ADR 0038 and its check (decision 7), handled as a process
   failure: first. Then the swaps of
   "hand-written-code-a-maintained-library-could-replace", in order.
2. T035, next on the critical path (`go run ./tools/ci plan
   --critical-path`), paused on 2026-10-20 until ADR 0038 is Accepted;
   its work is on the branch `feat/t035-memstore`, and its store lock
   and atomic writes are chosen under ADR 0038.
3. The docs pull request of
   "the-spec-catches-up-with-the-hardening-of-t127-t143-and-t144"
   lands; its one open question stays in that note.
4. The rest of the ready list (`go run ./tools/ci plan --ready`).
5. T112 lands through the sessions of the config and kit repositories
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
3. T111 (sitting M1) waits for T054, T059, T060, T078 and T035.
4. When to harden `gitsafe`'s fast-forward
   ("gitsafe-s-fast-forward-overwrites-ignored-files-and-runs"):
   a task on the `gitsafe` surface. Recommended: before `julieta setup`
   runs in daily use, since it can overwrite an ignored file unasked.
5. Paste the prompt of "library-first-in-the-sandbox-payload" into the
   session of the payload's source repository, so ADR 0038 holds in
   every repository. Recommended: today.

## In flight

- Nothing; the last session ended at this checkpoint.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
