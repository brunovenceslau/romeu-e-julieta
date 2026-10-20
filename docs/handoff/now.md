# Current state

## Where we stopped

Checkpoint of 2026-10-20, after T035 merged (Done record 46, which
ticks T035, T145 and T146).

## Next steps

1. The next swap of ADR 0038, the ssh-agent listing of `internal/signing`
   (signing surface, ask first), per
   "hand-written-code-a-maintained-library-could-replace".
2. Accept ADR 0039, with its supersedes lines for ADR 0001 (operator
   item 5).
3. The rest of the ready list (`go run ./tools/ci plan --ready`); T111
   waits for T054, T059, T060 and T078.
4. T112 lands through the sessions of the config and kit repositories
   ("t112-in-the-config-and-kit-repositories").

## Open for the operator

One block, a recommendation each. Decisions of 2026-10-19 are in
"the-grants-of-2026-10-19".

1. Paste the two T112 prompts, the config repository's first.
   Recommended: today, so the worktree hook lands before the next
   fan-out of subagents.
2. The sitting of T126 (O8) waits for the GitHub App of
   [ADR 0011, keep the administrator's pull-request bypass and give the sandbox a GitHub App token](../adr/0011-keep-the-administrator-s-pull-request-bypass-and-give-the-sandbox-a-github-app-token.md),
   which does not exist yet. Recommended: create it when M3 starts.
3. When to harden `gitsafe`'s fast-forward
   ("gitsafe-s-fast-forward-overwrites-ignored-files-and-runs"):
   a task on the `gitsafe` surface. Recommended: before `julieta setup`
   runs in daily use, since it can overwrite an ignored file unasked.
4. Whether the removal of a corrupted cache entry in the container e2e
   writes a line ("the-spec-catches-up-with-the-hardening-of-t127-t143-and-t144").
   Recommended: a line on a channel the infrastructure classifier does
   not read, such as a job annotation, so a poisoned cache shows.
5. Accept ADR 0039. Recommended: with its supersedes lines, in a
   one-line pull request.
6. ADR 0021, keep memory as plain per-repo files, is merged but still
   Proposed, and T035 now builds on it. Recommended: accept it, in a
   one-line pull request.
7. Paste the ADR 0039 prompt into the payload's session
   ("rules-to-move-when-the-interim-payload-is-retired"). Recommended:
   today.

## In flight

- Nothing besides this checkpoint.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
